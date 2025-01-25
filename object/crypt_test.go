package object

import (
	"bytes"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	ks := &Keyset{
		CryptKey:   [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
		HmacKey:    [32]byte{32, 31, 30, 29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1},
		BlobIdSalt: [32]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
	}

	tests := []struct {
		name    string
		input   []byte
		wantErr bool
	}{
		{"hello", []byte("hello world"), false},
		{"medium", bytes.Repeat([]byte("abc"), 100), false},
		{"large", bytes.Repeat([]byte("def"), 1000), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := Encrypt(tt.input, ks)
			if err != nil {
				t.Fatalf("Encrypt() error = %v", err)
			}

			decrypted, err := Decrypt(encrypted, ks)
			if err != nil {
				t.Fatalf("Decrypt() error = %v", err)
			}

			if !bytes.Equal(tt.input, decrypted) {
				t.Errorf("Round trip failed:\ngot  %x\nwant %x", decrypted, tt.input)
			}
		})
	}
}

func TestDecryptErrors(t *testing.T) {
	ks := &Keyset{
		CryptKey: [32]byte{1},
		HmacKey:  [32]byte{1},
	}

	tests := []struct {
		name    string
		input   []byte
		wantErr string
	}{
		{"empty input", []byte{}, "read header"},
		{"short input", []byte("ARQO"), "read header"},
		{"invalid input", make([]byte, 116), "magic mismatch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decrypt(tt.input, ks)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !bytes.Contains([]byte(err.Error()), []byte(tt.wantErr)) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, err)
			}
		})
	}
}
