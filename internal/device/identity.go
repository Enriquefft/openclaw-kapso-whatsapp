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
	"math/big"
	"os"
	"path/filepath"
	"time"
)

const keyFile = "device-key.pem"

// Identity holds a persistent ECDSA P-256 keypair used to prove device
// identity to the OpenClaw gateway.
type Identity struct {
	key *ecdsa.PrivateKey
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
		return &Identity{key: key}, nil
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

	return &Identity{key: key}, nil
}

// DeviceID returns a hex-encoded SHA-256 fingerprint of the public key.
func (id *Identity) DeviceID() string {
	pub, _ := x509.MarshalPKIXPublicKey(&id.key.PublicKey)
	h := sha256.Sum256(pub)
	return hex.EncodeToString(h[:])
}

// PublicKeyBase64 returns the base64-encoded DER public key.
func (id *Identity) PublicKeyBase64() string {
	pub, _ := x509.MarshalPKIXPublicKey(&id.key.PublicKey)
	return base64.StdEncoding.EncodeToString(pub)
}

// Sign signs the given nonce with the device private key and returns the
// base64-encoded ASN.1 DER signature plus the signing timestamp (Unix ms).
func (id *Identity) Sign(nonce string) (signature string, signedAt int64, err error) {
	h := sha256.Sum256([]byte(nonce))
	r, s, err := ecdsa.Sign(rand.Reader, id.key, h[:])
	if err != nil {
		return "", 0, fmt.Errorf("sign nonce: %w", err)
	}

	// ASN.1 DER encode (r, s).
	sig, err := asn1Signature(r, s)
	if err != nil {
		return "", 0, err
	}

	now := time.Now().UnixMilli()
	return base64.StdEncoding.EncodeToString(sig), now, nil
}

// asn1Signature produces a minimal ASN.1 DER SEQUENCE of two INTEGERs.
func asn1Signature(r, s *big.Int) ([]byte, error) {
	rb := intBytes(r)
	sb := intBytes(s)

	// SEQUENCE { INTEGER rb, INTEGER sb }
	inner := make([]byte, 0, 2+len(rb)+2+len(sb))
	inner = append(inner, 0x02, byte(len(rb)))
	inner = append(inner, rb...)
	inner = append(inner, 0x02, byte(len(sb)))
	inner = append(inner, sb...)

	out := make([]byte, 0, 2+len(inner))
	out = append(out, 0x30, byte(len(inner)))
	out = append(out, inner...)
	return out, nil
}

// intBytes returns the big-endian representation with a leading 0x00 if the
// high bit is set (ASN.1 INTEGER is signed).
func intBytes(v *big.Int) []byte {
	b := v.Bytes()
	if len(b) > 0 && b[0]&0x80 != 0 {
		b = append([]byte{0x00}, b...)
	}
	return b
}

func parseKey(data []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	return x509.ParseECPrivateKey(block.Bytes)
}
