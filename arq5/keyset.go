package arq5

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/Backblaze/blazer/b2"
	"github.com/element-hq/mautrix-go/crypto/pkcs7"
	"github.com/quad/arqfix/object"
	"golang.org/x/crypto/pbkdf2"
)

const KEYSET_HEADER = "ENCRYPTIONV2"
const PATH_KEYSET = "encryptionv3.dat"

type EncryptedKeyset struct {
	header     [len(KEYSET_HEADER)]byte
	salt       [8]byte
	hmac       [32]byte
	iv         [16]byte
	ciphertext []byte
}

func (eks *EncryptedKeyset) Read(r io.Reader) error {
	if err := binary.Read(r, binary.BigEndian, &eks.header); err != nil {
		return fmt.Errorf("header: %w", err)
	}

	if err := binary.Read(r, binary.BigEndian, &eks.salt); err != nil {
		return fmt.Errorf("salt: %w", err)
	}

	if err := binary.Read(r, binary.BigEndian, &eks.hmac); err != nil {
		return fmt.Errorf("hmac: %w", err)
	}

	if err := binary.Read(r, binary.BigEndian, &eks.iv); err != nil {
		return fmt.Errorf("iv: %w", err)
	}

	ciphertext, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("ciphertext: %w", err)
	}
	eks.ciphertext = ciphertext

	if string(eks.header[:]) != KEYSET_HEADER {
		return fmt.Errorf("header mismatch")
	}

	return nil
}

type Keyset struct {
	CryptKey   [32]byte
	HmacKey    [32]byte
	BlobIdSalt [32]byte
}

func (ks *Keyset) Read(eks *EncryptedKeyset, pass string) error {
	key := pbkdf2.Key([]byte(pass), eks.salt[:], 200000, 64, sha1.New)
	cryptKey := key[:32]
	hmacKey := key[32:]

	mac := hmac.New(sha256.New, hmacKey)
	mac.Write(eks.iv[:])
	mac.Write(eks.ciphertext)
	expectedHmac := mac.Sum(nil)

	if !hmac.Equal(eks.hmac[:], expectedHmac) {
		return fmt.Errorf("hmac mismatch")
	}

	block, err := aes.NewCipher(cryptKey)
	if err != nil {
		return fmt.Errorf("invalid key: %w", err)
	}

	mode := cipher.NewCBCDecrypter(block, eks.iv[:])
	plaintext := make([]byte, len(eks.ciphertext))
	mode.CryptBlocks(plaintext, eks.ciphertext)
	plaintext = pkcs7.Unpad(plaintext)

	r := bytes.NewReader(plaintext)
	if err := binary.Read(r, binary.BigEndian, ks); err != nil {
		return fmt.Errorf("read: %w", err)
	}

	return nil
}

func UnlockSet(ctx context.Context, bucket *b2.Bucket, set string, pass string) (*object.Keyset, error) {
	bs, err := object.ReadObject(ctx, bucket, set, PATH_KEYSET)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(bs)

	var eks EncryptedKeyset
	if err = eks.Read(reader); err != nil {
		return nil, fmt.Errorf("read encryptedkeyset: %w", err)
	}

	var ks Keyset
	if err = ks.Read(&eks, pass); err != nil {
		return nil, fmt.Errorf("read keyset: %w", err)
	}

	return &object.Keyset{
		CryptKey:   ks.CryptKey,
		HmacKey:    ks.HmacKey,
		BlobIdSalt: ks.BlobIdSalt,
	}, nil
}
