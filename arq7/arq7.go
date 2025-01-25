package arq7

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path"

	"github.com/Backblaze/blazer/b2"
	"github.com/quad/arqfix/object"
	"github.com/schollz/progressbar/v3"
)

type Record struct {
	Archived           bool
	ArqVersion         string
	BackupFolderUUID   string
	BackupPlanUUID     string
	CopiedFromCommit   bool
	CopiedFromSnapshot bool
	CreationDate       uint
	ErrorCount         uint
	IsComplete         bool
	LocalPath          string
	RelativePath       string
	Version            uint
}

const PATH_BACKUPFOLDERS = "backupfolders"
const PATH_BACKUPRECORDS = "backuprecords"
const PATH_BACKUPRECORD_EXTENSION = ".backuprecord"

func Records(ctx context.Context, bucket *b2.Bucket, plan string, folder Folder, ks *object.Keyset) ([]Record, error) {
	var records []Record
	paths, err := object.ListDescendantObjects(ctx, bucket, plan, PATH_BACKUPFOLDERS, folder.Uuid, PATH_BACKUPRECORDS)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}

	pb := progressbar.NewOptions(len(paths)+1, progressbar.OptionSetMaxDetailRow(1))
	for _, p := range paths {
		pb.Add(1)
		pb.AddDetail(p)

		if path.Ext(p) != PATH_BACKUPRECORD_EXTENSION {
			continue
		}

		ct, err := object.ReadObject(ctx, bucket, p)
		if err != nil {
			return nil, fmt.Errorf("read %v: %w", p, err)
		}

		pt, err := object.Decrypt(ct, ks)
		if err != nil {
			return nil, fmt.Errorf("decrypt %v: %w", p, err)
		}

		bs, err := object.Decompress(pt)
		if err != nil {
			return nil, fmt.Errorf("decompress %v: %w", p, err)
		}

		var record Record
		if err := json.Unmarshal(bs, &record); err != nil {
			return nil, fmt.Errorf("parse %v: %w", p, err)
		}

		records = append(records, record)
	}

	return records, nil
}

type Folder struct {
	LocalPath         string `json:"localPath"`
	MigratedFromArq60 bool   `json:"migratedFromArq60"`
	StorageClass      string `json:"storageClass"`
	DiskIdentifier    string `json:"diskIdentifier"`
	Uuid              string `json:"uuid"`
	MigratedFromArq5  bool   `json:"migratedFromArq5"`
	LocalMountPoint   string `json:"localMountPoint"`
	Name              string `json:"name"`
}

const PATH_BACKUPFOLDER = "backupfolder.json"

func Folders(ctx context.Context, bucket *b2.Bucket, plan string, ks *object.Keyset) ([]Folder, error) {
	var folders []Folder
	paths, err := object.ListDescendantObjects(ctx, bucket, plan, PATH_BACKUPFOLDERS)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}

	for _, p := range paths {
		if path.Base(p) != PATH_BACKUPFOLDER {
			continue
		}

		log.Println(p)

		ct, err := object.ReadObject(ctx, bucket, p)
		if err != nil {
			return nil, fmt.Errorf("read %v: %w", p, err)
		}

		pt, err := object.Decrypt(ct, ks)
		if err != nil {
			return nil, fmt.Errorf("decrypt %v: %w", p, err)
		}

		var folder Folder
		if err := json.Unmarshal(pt, &folder); err != nil {
			return nil, fmt.Errorf("parse %v: %w", p, err)
		}

		folders = append(folders, folder)
	}

	return folders, nil
}

func WriteFolder(ctx context.Context, bucket *b2.Bucket, plan string, folder Folder, ks *object.Keyset) error {
	bs, err := json.Marshal(folder)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	pt, err := object.Encrypt(bs, ks)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	if err := object.WriteObject(ctx, bucket, pt, plan, PATH_BACKUPFOLDERS, folder.Uuid, PATH_BACKUPFOLDER); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

func FolderExists(ctx context.Context, bucket *b2.Bucket, plan, folder string) (bool, error) {
	objPath := path.Join(plan, PATH_BACKUPFOLDERS, folder, PATH_BACKUPFOLDER)
	_, err := bucket.Object(objPath).Attrs(ctx)
	if b2.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("object attrs: %w", err)
	}

	return true, nil
}
