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

// Identity holds a persistent Ed25519 keypair used to prove device
// identity to the OpenClaw gateway during the v3 handshake.
type Identity struct {
	key ed25519.PrivateKey
	pub ed25519.PublicKey
}

// LoadOrCreate reads an existing device key from dir, or generates a new
// Ed25519 key and persists it. Incompatible keys (e.g. old ECDSA) are
// removed and regenerated automatically. The directory is created if needed.
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
		Type:  "ED25519 SEED",
		Bytes: priv.Seed(),
	})

	if err := os.WriteFile(path, pemBlock, 0o600); err != nil {
		return nil, fmt.Errorf("write device key %s: %w", path, err)
	}

	return &Identity{key: priv, pub: pub}, nil
}

// DeviceID returns a hex-encoded SHA-256 fingerprint of the public key.
func (id *Identity) DeviceID() string {
	h := sha256.Sum256(id.pub)
	return hex.EncodeToString(h[:])
}

// PublicKeyBase64 returns the raw public key encoded as base64url (no padding).
func (id *Identity) PublicKeyBase64() string {
	return base64.RawURLEncoding.EncodeToString(id.pub)
}

// Sign signs the given data with the device private key using Ed25519.
func (id *Identity) Sign(data []byte) []byte {
	return ed25519.Sign(id.key, data)
}

func parseKey(data []byte) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	if block.Type != "ED25519 SEED" {
		return nil, fmt.Errorf("unexpected PEM type %q", block.Type)
	}
	seed := block.Bytes
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("invalid Ed25519 seed length: %d", len(seed))
	}
	return ed25519.NewKeyFromSeed(seed), nil
}
