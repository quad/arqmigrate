package arq7

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"path"

	"github.com/Backblaze/blazer/b2"
	"github.com/quad/arqfix/arq5"
	"github.com/quad/arqfix/object"
)

type MigratedRecord struct {
	Archived           bool           `json:"archived"`
	Arq5BucketXML      string         `json:"arq5BucketXML"`
	Arq5TreeBlobKey    BlobKey        `json:"arq5TreeBlobKey"`
	ArqVersion         string         `json:"arqVersion"`
	BackupFolderUUID   string         `json:"backupFolderUUID"`
	BackupPlanUUID     string         `json:"backupPlanUUID"`
	ComputerOSType     ComputerOSType `json:"computerOSType"`
	CopiedFromCommit   bool           `json:"copiedFromCommit"`
	CopiedFromSnapshot bool           `json:"copiedFromSnapshot"`
	CreationDate       int64          `json:"creationDate"`
	ErrorCount         int            `json:"errorCount"`
	IsComplete         bool           `json:"isComplete"`
	LocalPath          string         `json:"localPath"`
	RelativePath       string         `json:"relativePath"`
	StorageClass       string         `json:"storageClass"`
	Version            uint           `json:"version"`
}

type BlobKey struct {
	ArchiveSize          uint                 `json:"archiveSize"`
	CompressionType      arq5.CompressionType `json:"compressionType"`
	Sha1                 string               `json:"sha1"`
	StorageType          StorageType          `json:"storageType"`
	StretchEncryptionKey bool                 `json:"stretchEncryptionKey"`
}

type StorageType int32

const (
	StorageTypeS3 StorageType = 1
	StorageTypeGlacier
	StorageTypeS3Glacier
)

type ComputerOSType int32

const (
	ComputerOSTypeMac ComputerOSType = 1
	ComputerOSTypeWindows
)

func MigrateFolder(f arq5.BackupFolder) (*Folder, error) {
	return &Folder{
		LocalPath:         f.LocalPath,
		MigratedFromArq60: false,
		StorageClass:      "STANDARD",
		DiskIdentifier:    "ROOT", // TODO: what is this?
		Uuid:              f.BucketUUID,
		MigratedFromArq5:  true,
		LocalMountPoint:   f.LocalMountPoint,
		Name:              html.UnescapeString(f.BucketName),
	}, nil
}

const RECORD_TIME_PREFIX_LENGTH = 5

func MigrateRecord(plan, folder string, r arq5.Commit) (*MigratedRecord, error) {
	var arqVersion string
	if r.ArqVersion == "" {
		arqVersion = "arqfix"
	} else {
		arqVersion = r.ArqVersion
	}

	var folderPath string
	url, err := url.Parse(r.FolderPath)
	if err != nil {
		return nil, fmt.Errorf("invalid FolderPath '%v'", r.FolderPath)
	}
	folderPath = url.Path

	time := fmt.Sprintf("%012d", r.CreationDate.Unix())
	timeHead := time[:RECORD_TIME_PREFIX_LENGTH]
	timeTail := time[RECORD_TIME_PREFIX_LENGTH:]
	relativePath := path.Join("/", plan, PATH_BACKUPFOLDERS, folder, PATH_BACKUPRECORDS, timeHead, timeTail+PATH_BACKUPRECORD_EXTENSION)

	return &MigratedRecord{
		Archived:      true,
		Arq5BucketXML: string(r.ConfigPlistXML),
		Arq5TreeBlobKey: BlobKey{
			ArchiveSize:          0,
			CompressionType:      r.TreeCompressionType,
			Sha1:                 r.TreeSha1,
			StorageType:          StorageTypeS3,
			StretchEncryptionKey: r.TreeEncryptionKeyStretched,
		},
		ArqVersion:         arqVersion,
		BackupFolderUUID:   folder,
		BackupPlanUUID:     plan,
		ComputerOSType:     ComputerOSTypeMac,
		CopiedFromCommit:   true,
		CopiedFromSnapshot: false,
		CreationDate:       r.CreationDate.Unix(),
		ErrorCount:         len(r.FailedFiles),
		IsComplete:         r.IsComplete,
		LocalPath:          folderPath,
		RelativePath:       relativePath,
		StorageClass:       "STANDARD",
		Version:            12,
	}, nil
}

func WriteMigratedRecord(ctx context.Context, bucket *b2.Bucket, r *MigratedRecord, ks *object.Keyset) error {
	bs, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	ct, err := object.Compress(bs)
	if err != nil {
		return fmt.Errorf("compress: %w", err)
	}

	enc, err := object.Encrypt(ct, ks)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	if err := object.WriteObject(ctx, bucket, enc, r.RelativePath); err != nil {
		return fmt.Errorf("write %v: %w", r.RelativePath, err)
	}

	return nil
}
