package device

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

const keyFile = "device-key.pem"

type Identity struct {
	key ed25519.PrivateKey
	pub ed25519.PublicKey
}

func LoadOrCreate(dir string) (*Identity, error) {
	path := filepath.Join(dir, keyFile)

	data, err := os.ReadFile(path)
	if err == nil {
		key, parseErr := parseKey(data)
		if parseErr != nil {
			log.Printf("device: removing incompatible key at %s (%v), generating new Ed25519 key", path, parseErr)
			_ = os.Remove(path)
		} else {
			return &Identity{key: key, pub: key.Public().(ed25519.PublicKey)}, nil
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read device key %s: %w", path, err)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate device key: %w", err)
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create state dir %s: %w", dir, err)
	}

	pemBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: priv.Seed(),
	})

	if err := os.WriteFile(path, pemBlock, 0o600); err != nil {
		return nil, fmt.Errorf("write device key %s: %w", path, err)
	}

	return &Identity{key: priv, pub: pub}, nil
}

func (id *Identity) DeviceID() string {
	h := sha256.Sum256(id.pub)
	return hex.EncodeToString(h[:])
}

func (id *Identity) PublicKeyBase64() string {
	return base64.RawURLEncoding.EncodeToString(id.pub)
}

func (id *Identity) Sign(data []byte) []byte {
	return ed25519.Sign(id.key, data)
}

func parseKey(data []byte) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	seed := block.Bytes
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("invalid Ed25519 seed length: %d", len(seed))
	}
	return ed25519.NewKeyFromSeed(seed), nil
}
