package object

import (
	"bytes"
	"testing"
)

func TestCompress(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantErr bool
	}{
		{
			name:    "empty input",
			input:   []byte{},
			wantErr: false,
		},
		{
			name:    "small input",
			input:   []byte("hello world"),
			wantErr: false,
		},
		{
			name:    "larger input",
			input:   bytes.Repeat([]byte("abcdef"), 100),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressed, err := Compress(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Compress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(compressed) <= 0 {
					t.Error("Compress() returned empty result")
				}
			}
		})
	}
}

func TestDecompress(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantErr bool
	}{
		{
			name:    "invalid input - too short",
			input:   []byte{0, 1, 2},
			wantErr: true,
		},
		{
			name:    "invalid input - wrong length",
			input:   []byte{0, 0, 0, 10, 1, 2, 3},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decompress(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Decompress() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCompressDecompressRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{
			name:  "empty",
			input: []byte{},
		},
		{
			name:  "small text",
			input: []byte("hello world"),
		},
		{
			name:  "repeated content",
			input: bytes.Repeat([]byte("abcdef"), 100),
		},
		{
			name:  "binary data",
			input: []byte{0, 1, 2, 3, 4, 5, 255, 254, 253},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressed, err := Compress(tt.input)
			if err != nil {
				t.Fatalf("Compress() error = %v", err)
			}

			decompressed, err := Decompress(compressed)
			if err != nil {
				t.Fatalf("Decompress() error = %v", err)
			}

			if !bytes.Equal(tt.input, decompressed) {
				t.Errorf("Round trip failed: input len=%d, output len=%d, data mismatch",
					len(tt.input), len(decompressed))
			}
		})
	}
}
