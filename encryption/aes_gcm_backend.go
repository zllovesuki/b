package encryption

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"time"

	"github.com/zllovesuki/b/app"

	"github.com/pkg/errors"
)

// AESGCM wraps an existing app.Backend and add AES-GCM mode encryption/decryption on top of it.
// AES-GCM mode is specified by the key length
type AESGCM struct {
	backend app.Backend
	key     []byte
}

var _ app.Backend = &AESGCM{}

// NewAESGCMBackend returns an AES-GCM mode transparent encryption wrapper. len(key) determines
// if operating in AES-128 (16), AES-192 (24), or AES-256 (32) mode.
func NewAESGCMBackend(backend app.Backend, key []byte) (*AESGCM, error) {
	if backend == nil {
		return nil, errors.New("missing backend")
	}
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, errors.New("invalid key length")
	}
	return &AESGCM{
		backend: backend,
		key:     key,
	}, nil
}

func (a *AESGCM) SaveTTL(c context.Context, identifier string, data []byte, ttl time.Duration) error {
	ciphertext, err := a.encrypt(data)
	if err != nil {
		return errors.Wrap(err, "encrypting during save")
	}
	return a.backend.SaveTTL(c, identifier, ciphertext, ttl)
}

func (a *AESGCM) Retrieve(c context.Context, identifier string) ([]byte, error) {
	ciphertext, err := a.backend.Retrieve(c, identifier)
	if err != nil {
		return nil, errors.Wrap(err, "getting ciphertext from backend")
	}
	return a.decrypt(ciphertext)
}

func (a *AESGCM) encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, errors.Wrap(err, "opening a cipher block")
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.Wrap(err, "opening aesgcm")
	}

	output := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, output); err != nil {
		return nil, errors.Wrap(err, "initializing IV")
	}

	b := aesgcm.Seal(output, output, data, nil)

	return b, nil
}

func (a *AESGCM) decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, errors.Wrap(err, "opening a cipher block")
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.Wrap(err, "opening aesgcm")
	}

	if len(data) < aesgcm.NonceSize()+aesgcm.Overhead() {
		return nil, errors.New("ciphertext too short")
	}

	c, err := aesgcm.Open(nil, data[:aesgcm.NonceSize()], data[aesgcm.NonceSize():], nil)
	if err != nil {
		return nil, errors.Wrap(err, "decrypting")
	}

	return c, nil
}

func (a *AESGCM) Close() error {
	return a.backend.Close()
}
