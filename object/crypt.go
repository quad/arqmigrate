package object

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"log"

	"github.com/element-hq/mautrix-go/crypto/pkcs7"
)

type Keyset struct {
	CryptKey   [32]byte
	HmacKey    [32]byte
	BlobIdSalt [32]byte
}

type encryptedObject struct {
	Magic                     [4]byte
	Hmac                      [32]byte
	MasterIV                  [16]byte
	EncryptedDataIVSessionKey [64]byte
}

type metadata struct {
	DataIV     [16]byte
	SessionKey [32]byte
}

func Decrypt(bs []byte, ks *Keyset) ([]byte, error) {
	r := bytes.NewReader(bs)

	var eo encryptedObject
	if err := binary.Read(r, binary.BigEndian, &eo); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	if string(eo.Magic[:]) != "ARQO" {
		return nil, fmt.Errorf("magic mismatch")
	}

	ciphertext, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read ciphertext: %w", err)
	}

	mac := hmac.New(sha256.New, ks.HmacKey[:])
	mac.Write(eo.MasterIV[:])
	mac.Write(eo.EncryptedDataIVSessionKey[:])
	mac.Write(ciphertext)
	expectedHmac := mac.Sum(nil)

	if !hmac.Equal(eo.Hmac[:], expectedHmac) {
		return nil, fmt.Errorf("hmac mismatch (actual %x != expected %x)", eo.Hmac[:], expectedHmac)
	}

	block, err := aes.NewCipher(ks.CryptKey[:])
	if err != nil {
		return nil, fmt.Errorf("invalid crypt key: %w", err)
	}

	mode := cipher.NewCBCDecrypter(block, eo.MasterIV[:])
	plaintext := make([]byte, len(eo.EncryptedDataIVSessionKey))
	mode.CryptBlocks(plaintext, eo.EncryptedDataIVSessionKey[:])
	plaintext = pkcs7.Unpad(plaintext)

	var m metadata
	if err := binary.Read(bytes.NewReader(plaintext), binary.BigEndian, &m); err != nil {
		return nil, fmt.Errorf("read metadata: %w", err)
	}

	block, err = aes.NewCipher(m.SessionKey[:])
	if err != nil {
		return nil, fmt.Errorf("invalid session key: %w", err)
	}

	mode = cipher.NewCBCDecrypter(block, m.DataIV[:])
	plaintext = make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)
	plaintext = pkcs7.Unpad(plaintext)

	return plaintext, nil
}

func randomBytes(n int) []byte {
	bs := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, bs); err != nil {
		log.Fatal("read random:", err)
	}
	return bs
}

func Encrypt(bs []byte, ks *Keyset) ([]byte, error) {
	dataIV := randomBytes(16)
	sessionKey := randomBytes(32)

	block, err := aes.NewCipher(sessionKey)
	if err != nil {
		return nil, fmt.Errorf("invalid session key: %w", err)
	}

	paddedData := pkcs7.Pad(bs, aes.BlockSize)
	mode := cipher.NewCBCEncrypter(block, dataIV)
	ciphertext := make([]byte, len(paddedData))
	mode.CryptBlocks(ciphertext, paddedData)

	metadata := metadata{
		DataIV:     *(*[16]byte)(dataIV),
		SessionKey: *(*[32]byte)(sessionKey),
	}

	metadataBytes := new(bytes.Buffer)
	if err := binary.Write(metadataBytes, binary.BigEndian, metadata); err != nil {
		return nil, fmt.Errorf("write metadata: %w", err)
	}

	paddedMetadata := pkcs7.Pad(metadataBytes.Bytes(), aes.BlockSize)
	if len(paddedMetadata) != 64 {
		return nil, fmt.Errorf("invalid padded metadata length: got %d, want 64", len(paddedMetadata))
	}

	block, err = aes.NewCipher(ks.CryptKey[:])
	if err != nil {
		return nil, fmt.Errorf("invalid crypt key: %w", err)
	}

	masterIV := randomBytes(16)
	mode = cipher.NewCBCEncrypter(block, masterIV)
	encryptedDataIVSessionKey := make([]byte, len(paddedMetadata))
	mode.CryptBlocks(encryptedDataIVSessionKey, paddedMetadata)

	mac := hmac.New(sha256.New, ks.HmacKey[:])
	mac.Write(masterIV)
	mac.Write(encryptedDataIVSessionKey)
	mac.Write(ciphertext)
	hmacSum := mac.Sum(nil)

	eo := encryptedObject{
		Magic:                     [4]byte{'A', 'R', 'Q', 'O'},
		Hmac:                      *(*[32]byte)(hmacSum),
		MasterIV:                  *(*[16]byte)(masterIV),
		EncryptedDataIVSessionKey: *(*[64]byte)(encryptedDataIVSessionKey),
	}

	eoBytes := new(bytes.Buffer)
	if err := binary.Write(eoBytes, binary.BigEndian, eo); err != nil {
		return nil, fmt.Errorf("write header: %w", err)
	}

	return append(eoBytes.Bytes(), ciphertext...), nil
}
