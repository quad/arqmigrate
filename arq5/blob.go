package arq5

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Backblaze/blazer/b2"
	"github.com/quad/arqfix/object"
)

// PackIndex represents the structure of a pack index file
type PackIndex struct {
	Magic   [4]byte
	Version uint32
	Fanout  [256]uint32
	Objects []PackIndexEntry
	SHA1    [20]byte
}

type PackIndexEntry struct {
	Offset  uint64
	Length  uint64
	SHA1    [20]byte
	Padding [4]byte
}

// PackFile represents the structure of a pack file
type PackFile struct {
	Signature [4]byte
	Version   uint32
	Objects   []PackObject `arq5:"len64"`
	SHA1      [20]byte
}

type PackObject struct {
	MimeType string
	Name     string
	Data     []byte `arq5:"len64"`
}

func ReadTreeBlob(ctx context.Context, bucket *b2.Bucket, computerUUID string, folderUUID string, sha string) ([]byte, error) {
	data, err := findInPacks(ctx, bucket, computerUUID, folderUUID, sha)
	if err == nil {
		return data, nil
	}

	paths := []string{
		path.Join(computerUUID, "objects", sha),
		path.Join(computerUUID, "objects", sha[:2], sha[2:]),
	}

	for _, path := range paths {
		data, err := object.ReadObject(ctx, bucket, path)
		if err == nil {
			return data, nil
		}
	}

	return nil, fmt.Errorf("treeblob %s not found", sha)
}

const PACK_READ_BATCH_SIZE = 10

func findInPacks(ctx context.Context, bucket *b2.Bucket, computerUUID string, folderUUID string, sha string) ([]byte, error) {
	objs, err := object.ListChildObjects(ctx, bucket, computerUUID, "packsets", folderUUID+"-trees")
	if err != nil {
		return nil, fmt.Errorf("list packsets: %w", err)
	}

	// Get all the index file names
	var indexNames []string
	for _, name := range objs {
		if filepath.Ext(name) == ".index" {
			indexNames = append(indexNames, name)
		}
	}

	// Pre-read all the index files
	wg := sync.WaitGroup{}
	c := make(chan string, PACK_READ_BATCH_SIZE)
	for i := 0; i < PACK_READ_BATCH_SIZE; i++ {
		wg.Add(1)

		go func() {
			for name := range c {
				if _, err := object.ReadObject(ctx, bucket, name); err != nil {
					continue
				}
			}
			wg.Done()
		}()
	}

	for _, name := range indexNames {
		c <- name
	}
	close(c)
	wg.Wait()

	// Search for the blob in the index files
	needle, err := hex.DecodeString(sha)
	if err != nil {
		return nil, fmt.Errorf("invalid SHA '%v': %w", sha, err)
	} else if len(needle) != 20 {
		return nil, fmt.Errorf("SHA '%v' has wrong length %d", sha, len(needle))
	}

	for _, name := range indexNames {
		indexData, err := object.ReadObject(ctx, bucket, name)
		if err != nil {
			return nil, fmt.Errorf("read packindex %v: %w", name, err)
		}

		index, err := parsePackIndex(indexData)
		if err != nil {
			return nil, fmt.Errorf("parse packindex %v: %w", name, err)
		}

		entry := findSHAInIndex(index, needle)
		if entry == nil {
			continue
		}

		packPath := strings.TrimSuffix(name, filepath.Ext(name)) + ".pack"
		packData, err := object.ReadObject(ctx, bucket, packPath)
		if err != nil {
			return nil, fmt.Errorf("read pack file %s: %w", packPath, err)
		}

		return readObjectFromPack(packData, entry.Offset, entry.Length)
	}

	return nil, fmt.Errorf("blob %s not found", sha)
}

func parsePackIndex(data []byte) (*PackIndex, error) {
	var index PackIndex

	r := bytes.NewReader(data)

	if err := binary.Read(r, binary.BigEndian, &index.Magic); err != nil {
		return nil, fmt.Errorf("read packindex magic")
	}
	if string(index.Magic[:]) != "\xff\x74\x4f\x63" {
		return nil, fmt.Errorf("invalid pack index magic")
	}

	if err := binary.Read(r, binary.BigEndian, &index.Version); err != nil {
		return nil, fmt.Errorf("read packindex version")
	}
	if index.Version != 2 {
		return nil, fmt.Errorf("unsupported pack index version: %d", index.Version)
	}

	if err := binary.Read(r, binary.BigEndian, &index.Fanout); err != nil {
		return nil, fmt.Errorf("read packindex fanout")
	}

	index.Objects = make([]PackIndexEntry, index.Fanout[255])
	if err := binary.Read(r, binary.BigEndian, &index.Objects); err != nil {
		return nil, fmt.Errorf("read packindex fanout")
	}

	if err := binary.Read(r, binary.BigEndian, &index.SHA1); err != nil {
		return nil, fmt.Errorf("read packindex SHA")
	}
	if sha := sha1.Sum(data[:len(data)-len(index.SHA1)]); index.SHA1 != sha {
		return nil, fmt.Errorf("invalid pack index SHA")
	}

	return &index, nil
}

func findSHAInIndex(index *PackIndex, needle []byte) *PackIndexEntry {
	fanoutIndex := needle[0]
	start := int(index.Fanout[fanoutIndex-1])
	end := int(index.Fanout[fanoutIndex])

	i, found := sort.Find(end-start, func(i int) int {
		return bytes.Compare(needle, index.Objects[start+i].SHA1[:])
	})

	if found {
		return &index.Objects[int(start)+i]
	}

	return nil
}

func readObjectFromPack(data []byte, offset uint64, length uint64) ([]byte, error) {
	r := bytes.NewReader(data)

	// Skip pack header
	var header struct {
		Signature [4]byte
		Version   uint32
		Count     uint64
	}
	if err := binary.Read(r, binary.BigEndian, &header); err != nil {
		return nil, err
	}
	if string(header.Signature[:]) != "PACK" {
		return nil, fmt.Errorf("invalid pack file signature")
	}

	// Seek to object
	if _, err := r.Seek(int64(offset), 0); err != nil {
		return nil, err
	}

	// Read object header and data
	var obj PackObject
	d := NewDecoder(r)
	if err := d.Decode(&obj); err != nil {
		return nil, fmt.Errorf("decode packobject: %w", err)
	}

	if len(obj.Data) != int(length) {
		return nil, fmt.Errorf("invalid pack file object length")
	}

	return obj.Data, nil
}
