package object

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/Backblaze/blazer/b2"
)

type ondemandReader struct {
	provider func() (io.Reader, error)
	reader   io.Reader
}

func newOndemandReader(provider func() (io.Reader, error)) *ondemandReader {
	return &ondemandReader{provider: provider, reader: nil}
}

func (r *ondemandReader) Read(p []byte) (n int, err error) {
	if r.reader == nil {
		reader, err := r.provider()
		if err != nil {
			return 0, err
		}

		r.reader = reader
	}

	return r.reader.Read(p)
}

func ReadObject(ctx context.Context, bucket *b2.Bucket, elem ...string) ([]byte, error) {
	objPath := path.Join(elem...)

	or := bucket.Object(objPath).NewReader(ctx)
	cr := NewCachingReader(or, objPath)
	defer cr.Close()

	bytes, err := io.ReadAll(cr)
	if err != nil {
		return nil, fmt.Errorf("read object %v: %w", objPath, err)
	}

	return bytes, nil
}

func ListChildObjects(ctx context.Context, bucket *b2.Bucket, elem ...string) ([]string, error) {
	objPath := path.Join(elem...) + "/"
	opts := []b2.ListOption{b2.ListDelimiter("/"), b2.ListPrefix(objPath)}

	return listObjects(ctx, bucket, opts, objPath)
}

func ListDescendantObjects(ctx context.Context, bucket *b2.Bucket, elem ...string) ([]string, error) {
	objPath := path.Join(elem...) + "/"
	opts := []b2.ListOption{b2.ListPrefix(objPath)}

	return listObjects(ctx, bucket, opts, objPath)
}

func listObjects(ctx context.Context, bucket *b2.Bucket, opts []b2.ListOption, cacheKey string) ([]string, error) {
	or := newOndemandReader(func() (io.Reader, error) {
		iter := bucket.List(ctx, opts...)
		objects, err := collectIter(iter)
		if err != nil {
			return nil, fmt.Errorf("list objects: %w", err)
		}

		return strings.NewReader(strings.Join(objects, "\n")), nil
	})
	cr := NewCachingReader(or, "list:"+cacheKey)

	return collectLines(cr)
}

func collectIter(iter *b2.ObjectIterator) ([]string, error) {
	var objects []string
	for iter.Next() {
		objects = append(objects, iter.Object().Name())
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}

	return objects, nil
}

func collectLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

func WriteObject(ctx context.Context, bucket *b2.Bucket, data []byte, elem ...string) error {
	objPath := strings.TrimPrefix(path.Join(elem...), "/")
	w := bucket.Object(objPath).NewWriter(ctx)
	defer w.Close()

	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write object %v: %w", objPath, err)
	}

	return nil
}
