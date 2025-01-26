package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/Backblaze/blazer/b2"
	"github.com/quad/arqfix/arq5"
	"github.com/quad/arqfix/arq7"
	"github.com/quad/arqfix/object"
)

func main() {
	bucketName := flag.String("bucket", "", "B2 bucket name")
	setName := flag.String("set", "", "Arq backup set name")
	passphraseFile := flag.String("passphrase-file", "", "File containing passphrase for the backup set")
	count := flag.Int("count", 0, "Maximum number of records to migrate")
	force := flag.Bool("force", false, "Force migration of records even if they already exist in the destination bucket")
	flag.Parse()

	if *bucketName == "" || *setName == "" || *passphraseFile == "" || *count == -1 {
		log.Fatalln("All flags (--bucket, --set, --passphrase-file, --count) are required")
	}

	passphraseBytes, err := os.ReadFile(*passphraseFile)
	if err != nil {
		log.Fatalln("Failed to read passphrase file:", err)
	}
	passphrase := strings.TrimSpace(string(passphraseBytes))

	ctx := context.Background()

	client, err := object.NewClient(ctx)
	if err != nil {
		log.Fatalln("client:", err)
	}

	bucket, err := client.Bucket(ctx, *bucketName)
	if err != nil {
		log.Fatalln("bucket:", err)
	}

	ks, err := arq5.UnlockSet(ctx, bucket, *setName, passphrase)
	if err != nil {
		log.Fatalln("unlock:", err)
	}

	for f, err := range arq5.Folders(ctx, bucket, *setName, ks) {
		if err != nil {
			log.Fatalln("folders:", err)
		}

		mf, err := arq7.MigrateFolder(f)
		if err != nil {
			log.Fatalln("migrate folder:", err)
		}

		if exists, err := arq7.FolderExists(ctx, bucket, f.ComputerUUID, mf.Uuid); err != nil {
			log.Fatal("folder exists:", err)
		} else if exists && !*force {
			log.Println("Folder:", mf.Name, mf.Uuid, "already exists, skipping. Use --force to override.")
		} else {
			if err := arq7.WriteFolder(ctx, bucket, f.ComputerUUID, *mf, ks); err != nil {
				log.Fatalln("write folder:", err)
			}
			log.Println("Folder:", mf.Name, mf.Uuid, "written")
		}

		for c, err := range arq5.Commits(ctx, bucket, f, ks) {
			if err != nil {
				log.Fatalln("records:", err)
			}

			if *count <= 0 {
				break
			}
			*count--

			mr, err := arq7.MigrateRecord(f.ComputerUUID, f.BucketUUID, c)
			if err != nil {
				log.Fatalln("migrate record:", err)
			}

			_, err = bucket.Object(strings.TrimPrefix(mr.RelativePath, "/")).Attrs(ctx)
			if b2.IsNotExist(err) || *force {
				if err := arq7.WriteMigratedRecord(ctx, bucket, mr, ks); err != nil {
					log.Fatal("write record:", err)
				}
				log.Println("Record:", mr.CreationDate, "written")
			} else if err != nil {
				log.Fatalln("record exists:", err)
			} else {
				log.Println("Record:", mr.CreationDate, "already exists, skipping. Use --force to override.")
			}
		}
	}

	return
}
