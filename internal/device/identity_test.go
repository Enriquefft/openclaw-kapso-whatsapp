package device

import (
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrCreateGeneratesKey(t *testing.T) {
	dir := t.TempDir()
	id, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("LoadOrCreate() error: %v", err)
	}
	if id.key == nil {
		t.Fatal("key should not be nil")
	}
	path := filepath.Join(dir, keyFile)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("key file not written: %v", err)
	}
}

func TestLoadOrCreateReusesKey(t *testing.T) {
	dir := t.TempDir()
	id1, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("first LoadOrCreate: %v", err)
	}
	id2, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("second LoadOrCreate: %v", err)
	}
	if id1.DeviceID() != id2.DeviceID() {
		t.Errorf("device IDs differ: %s vs %s", id1.DeviceID(), id2.DeviceID())
	}
}

func TestDeviceIDDeterministic(t *testing.T) {
	dir := t.TempDir()
	id, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}
	a := id.DeviceID()
	b := id.DeviceID()
	if a != b {
		t.Errorf("DeviceID not deterministic: %q vs %q", a, b)
	}
	if len(a) != 64 {
		t.Errorf("DeviceID should be 64 hex chars, got %d", len(a))
	}
}

func TestPublicKeyBase64Decodable(t *testing.T) {
	dir := t.TempDir()
	id, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}
	b64 := id.PublicKeyBase64()
	pub, err := base64.RawURLEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("base64url decode: %v", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		t.Fatalf("expected %d byte public key, got %d", ed25519.PublicKeySize, len(pub))
	}
}

func TestSignProducesVerifiableSignature(t *testing.T) {
	dir := t.TempDir()
	id, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}
	payload := []byte("v3|test|client|backend|operator|operator.read,operator.write|1234567890|token|nonce|darwin|")
	sig := id.Sign(payload)
	if !ed25519.Verify(id.pub, payload, sig) {
		t.Fatal("signature verification failed")
	}
}

func TestLoadOrCreateRegeneratesCorruptKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, keyFile)
	if err := os.WriteFile(path, []byte("not a pem file"), 0o600); err != nil {
		t.Fatal(err)
	}
	id, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("expected auto-regeneration, got error: %v", err)
	}
	if id.key == nil {
		t.Fatal("key should not be nil after regeneration")
	}
}
