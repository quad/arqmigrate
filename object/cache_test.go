package object

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestCachingReader(t *testing.T) {
	const testData = "hello world"
	const testKey = "test-key"

	if err := os.MkdirAll("cache", 0755); err != nil {
		t.Fatalf("failed to create cache directory: %v", err)
	}
	defer os.RemoveAll("cache")

	t.Run("basic read", func(t *testing.T) {
		r := NewCachingReader(strings.NewReader(testData), testKey)
		defer r.Close()

		buf := make([]byte, len(testData))
		n, err := io.ReadFull(r, buf)
		if err != nil {
			t.Fatalf("read failed: %v", err)
		}
		if n != len(testData) || string(buf) != testData {
			t.Errorf("got %q, want %q", string(buf), testData)
		}
	})

	t.Run("multiple reads", func(t *testing.T) {
		r := NewCachingReader(strings.NewReader(testData), testKey)
		defer r.Close()

		buf := make([]byte, 5)
		n, err := io.ReadFull(r, buf)
		if err != nil {
			t.Fatalf("first read failed: %v", err)
		}
		if n != 5 || string(buf) != "hello" {
			t.Errorf("first read: got %q, want %q", string(buf), "hello")
		}

		n, err = io.ReadFull(r, buf)
		if err != nil {
			t.Fatalf("second read failed: %v", err)
		}
		if n != 5 || string(buf) != " worl" {
			t.Errorf("second read: got %q, want %q", string(buf), " worl")
		}
	})

	t.Run("cached read", func(t *testing.T) {
		r1 := NewCachingReader(strings.NewReader(testData), testKey)
		if _, err := io.ReadAll(r1); err != nil {
			t.Fatalf("first read failed: %v", err)
		}
		r1.Close()

		r2 := NewCachingReader(strings.NewReader("wrong data"), testKey)
		defer r2.Close()

		data, err := io.ReadAll(r2)
		if err != nil {
			t.Fatalf("cached read failed: %v", err)
		}
		if string(data) != testData {
			t.Errorf("cached read: got %q, want %q", string(data), testData)
		}
	})

	t.Run("clear cache", func(t *testing.T) {
		r := NewCachingReader(strings.NewReader(testData), testKey)
		if _, err := io.ReadAll(r); err != nil {
			t.Fatalf("read failed: %v", err)
		}

		if err := r.ClearCache(); err != nil {
			t.Fatalf("clear cache failed: %v", err)
		}

		if _, err := os.Stat(r.cacheFile); !os.IsNotExist(err) {
			t.Error("cache file still exists after clear")
		}
		r.Close()
	})
}
