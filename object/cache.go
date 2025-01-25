package object

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sync"
)

const CACHE_DIRECTORY = "cache"

type CachingReader struct {
	reader    io.Reader
	mu        sync.Mutex
	cacheFile string
	file      *os.File
}

func keyToFilename(key string) string {
	sum := sha256.Sum256([]byte(key))
	sumString := hex.EncodeToString(sum[:])
	return filepath.Join(CACHE_DIRECTORY, sumString)
}

func NewCachingReader(reader io.Reader, cacheKey string) *CachingReader {
	return &CachingReader{
		reader:    reader,
		cacheFile: keyToFilename(cacheKey),
	}
}

func (c *CachingReader) Read(p []byte) (n int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.file == nil {
		if _, err := os.Stat(c.cacheFile); os.IsNotExist(err) {
			tmp, err := os.CreateTemp(CACHE_DIRECTORY, "partial-*")
			if err != nil {
				return 0, err
			}

			defer tmp.Close()
			defer os.Remove(tmp.Name())

			if _, err = io.Copy(tmp, c.reader); err != nil {
				return 0, err
			}

			if err = os.Rename(tmp.Name(), c.cacheFile); err != nil {
				return 0, err
			}
		}

		c.file, err = os.Open(c.cacheFile)
		if err != nil {
			return 0, err
		}
	}

	return c.file.Read(p)
}

func (c *CachingReader) Close() error {
	if c.file != nil {
		c.file.Close()
	}
	if closer, ok := c.reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (c *CachingReader) ClearCache() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.file != nil {
		c.file.Close()
		c.file = nil
	}
	return os.Remove(c.cacheFile)
}
