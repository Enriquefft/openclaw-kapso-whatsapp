package device

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const keyFile = "device-key.pem"

// Identity holds a persistent ECDSA P-256 keypair used to prove device
// identity to the OpenClaw gateway.
type Identity struct {
	key    *ecdsa.PrivateKey
	pubDER []byte // pre-computed DER-encoded public key
}

// LoadOrCreate reads an existing device key from dir, or generates a new
// P-256 key and persists it. The directory is created if it does not exist.
func LoadOrCreate(dir string) (*Identity, error) {
	path := filepath.Join(dir, keyFile)

	data, err := os.ReadFile(path)
	if err == nil {
		key, parseErr := parseKey(data)
		if parseErr != nil {
			return nil, fmt.Errorf("parse device key %s: %w", path, parseErr)
		}
		return newIdentity(key)
	}

	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read device key %s: %w", path, err)
	}

	// Generate new key.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate device key: %w", err)
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create state dir %s: %w", dir, err)
	}

	derBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal device key: %w", err)
	}

	pemBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: derBytes,
	})

	if err := os.WriteFile(path, pemBlock, 0o600); err != nil {
		return nil, fmt.Errorf("write device key %s: %w", path, err)
	}

	return newIdentity(key)
}

func newIdentity(key *ecdsa.PrivateKey) (*Identity, error) {
	pub, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal public key: %w", err)
	}
	return &Identity{key: key, pubDER: pub}, nil
}

// DeviceID returns a hex-encoded SHA-256 fingerprint of the public key.
func (id *Identity) DeviceID() string {
	h := sha256.Sum256(id.pubDER)
	return hex.EncodeToString(h[:])
}

// PublicKeyBase64 returns the base64-encoded DER public key.
func (id *Identity) PublicKeyBase64() string {
	return base64.StdEncoding.EncodeToString(id.pubDER)
}

// Sign signs the given nonce with the device private key and returns the
// base64-encoded ASN.1 DER signature plus the signing timestamp (Unix ms).
func (id *Identity) Sign(nonce string) (signature string, signedAt int64, err error) {
	h := sha256.Sum256([]byte(nonce))
	sig, err := ecdsa.SignASN1(rand.Reader, id.key, h[:])
	if err != nil {
		return "", 0, fmt.Errorf("sign nonce: %w", err)
	}

	now := time.Now().UnixMilli()
	return base64.StdEncoding.EncodeToString(sig), now, nil
}

func parseKey(data []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	return x509.ParseECPrivateKey(block.Bytes)
}
