package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/quad/arqfix/arq5"
	"github.com/quad/arqfix/object"
)

func main() {
	bucketName := flag.String("bucket", "", "B2 bucket name")
	setName := flag.String("set", "", "Arq backup set name")
	passphraseFile := flag.String("passphrase-file", "", "File containing passphrase for the backup set")
	flag.Parse()

	if *bucketName == "" || *setName == "" || *passphraseFile == "" {
		log.Fatalln("All flags (--bucket, --set, --passphrase-file) are required")
	}

	passphraseBytes, err := os.ReadFile(*passphraseFile)
	if err != nil {
		log.Fatalln("Failed to read passphrase file:", err)
	}
	passphrase := strings.TrimSpace(string(passphraseBytes))

	ctx := context.Background()

	client, err := object.NewClient(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	bucket, err := client.Bucket(ctx, *bucketName)
	if err != nil {
		log.Fatalln(err)
	}

	ks, err := arq5.UnlockSet(ctx, bucket, *setName, passphrase)
	if err != nil {
		log.Fatalln(err)
	}

	folders, err := arq5.Folders(ctx, bucket, *setName, ks)
	if err != nil {
		log.Fatalln(err)
	}

	for _, f := range folders {
		log.Printf("%+v", f)

		records, err := arq5.Commits(ctx, bucket, f, ks)
		if err != nil {
			log.Fatalln(err)
		}

		for _, r := range records {
			log.Printf("%+v", r)
		}
	}
}
