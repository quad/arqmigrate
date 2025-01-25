package arq5

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/Backblaze/blazer/b2"
	"github.com/quad/arqfix/object"
	"howett.net/plist"
)

type BackupFolder struct {
	AWSRegionName    string
	BucketUUID       string
	BucketName       string
	ComputerUUID     string
	LocalPath        string
	LocalMountPoint  string
	StorageType      uint64
	VaultName        string
	VaultCreatedTime float64
	Excludes         Exclusions
}

type Exclusions struct {
	Excludes   []string
	Enabled    bool
	MatchAny   bool
	Conditions []string
}

const PATH_BUCKETS = "buckets"

func Folders(ctx context.Context, bucket *b2.Bucket, plan string, ks *object.Keyset) ([]BackupFolder, error) {
	var folders []BackupFolder
	paths, err := object.ListChildObjects(ctx, bucket, plan, PATH_BUCKETS)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}

	for _, path := range paths {
		ct, err := object.ReadObject(ctx, bucket, path)
		if err != nil {
			return nil, fmt.Errorf("read %v: %w", path, err)
		}

		if string(ct[:9]) != "encrypted" {
			return nil, fmt.Errorf("folder %v is not encrypted", path)
		}

		data, err := object.Decrypt(ct[9:], ks)
		if err != nil {
			return nil, fmt.Errorf("decrypt %v: %w", path, err)
		}

		var folder BackupFolder
		if _, err := plist.Unmarshal(data, &folder); err != nil {
			return nil, fmt.Errorf("unmarshal %v: %w", path, err)
		}

		folders = append(folders, folder)
	}

	return folders, nil
}

const COMMIT_HEADER = "CommitV012"

type Commit struct {
	Author                     string
	Comment                    string
	ParentCommits              []ParentCommit `arq5:"len64"`
	TreeSha1                   string
	TreeEncryptionKeyStretched bool
	TreeCompressionType        CompressionType
	FolderPath                 string
	CreationDate               time.Time
	FailedFiles                []FailedFile `arq5:"len64"`
	HasMissingNodes            bool
	IsComplete                 bool
	ConfigPlistXML             []byte `arq5:"len64"`
	ArqVersion                 string
}

type ParentCommit struct {
	Sha1                   string
	EncryptionKeyStretched bool
}

type FailedFile struct {
	RelativePath string
	ErrorMessage string
}

const PATH_BUCKETDATA = "bucketdata"
const PATH_REFS_HEADS = "refs/heads/master"

func Commits(ctx context.Context, bucket *b2.Bucket, folder BackupFolder, ks *object.Keyset) ([]Commit, error) {
	refPath := path.Join(folder.ComputerUUID, PATH_BUCKETDATA, folder.BucketUUID, PATH_REFS_HEADS)

	refData, err := object.ReadObject(ctx, bucket, refPath)
	if err != nil {
		return nil, fmt.Errorf("read master ref %v: %w", refPath, err)
	}

	latestCommitSHA := strings.TrimSuffix(string(refData), "Y")

	commits := []Commit{}

	currentSHA := latestCommitSHA
	for currentSHA != "" {
		commit, err := readCommit(ctx, bucket, folder.ComputerUUID, folder.BucketUUID, currentSHA, ks)
		if err != nil {
			return nil, fmt.Errorf("read commit %s: %w", currentSHA, err)
		}

		commits = append(commits, *commit)

		if len(commit.ParentCommits) > 0 {
			currentSHA = commit.ParentCommits[0].Sha1
		} else {
			currentSHA = ""
		}
	}

	return commits, nil
}

func readCommit(ctx context.Context, bucket *b2.Bucket, computerUUID, folderUUID, sha string, ks *object.Keyset) (*Commit, error) {
	ct, err := ReadTreeBlob(ctx, bucket, computerUUID, folderUUID, sha)
	if err != nil {
		return nil, fmt.Errorf("read commit object: %w", err)
	}

	data, err := object.Decrypt(ct, ks)
	if err != nil {
		return nil, fmt.Errorf("decrypt commit: %w", err)
	}

	head := data[:len(COMMIT_HEADER)]
	rest := data[len(COMMIT_HEADER):]

	if string(head) != COMMIT_HEADER {
		return nil, fmt.Errorf("invalid commit header")
	}

	var commit Commit
	if err := Unmarshal(rest, &commit); err != nil {
		return nil, fmt.Errorf("unmarshal commit: %w", err)
	}

	return &commit, nil
}
