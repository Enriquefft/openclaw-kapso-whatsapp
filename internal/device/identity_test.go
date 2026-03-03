package device

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

// TestLoadOrCreateGeneratesKey verifies that LoadOrCreate creates a new key
// when none exists and that the key file is persisted.
func TestLoadOrCreateGeneratesKey(t *testing.T) {
	dir := t.TempDir()

	id, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("LoadOrCreate() error: %v", err)
	}

	if id.key == nil {
		t.Fatal("key should not be nil")
	}

	// Key file should exist on disk.
	path := filepath.Join(dir, keyFile)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("key file not written: %v", err)
	}
}

// TestLoadOrCreateReusesKey verifies that calling LoadOrCreate twice returns
// the same key — persistence is working, not regenerating each time.
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
		t.Errorf("device IDs differ: %s vs %s — key was regenerated", id1.DeviceID(), id2.DeviceID())
	}
}

// TestDeviceIDDeterministic verifies DeviceID is stable across calls.
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
		t.Errorf("DeviceID should be 64 hex chars (SHA-256), got %d", len(a))
	}
}

// TestPublicKeyBase64Decodable verifies that PublicKeyBase64 returns a valid
// base64-encoded DER public key that can be parsed back.
func TestPublicKeyBase64Decodable(t *testing.T) {
	dir := t.TempDir()
	id, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}

	b64 := id.PublicKeyBase64()
	der, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}

	pub, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		t.Fatalf("parse public key: %v", err)
	}

	if _, ok := pub.(*ecdsa.PublicKey); !ok {
		t.Fatalf("expected *ecdsa.PublicKey, got %T", pub)
	}
}

// TestSignProducesVerifiableSignature verifies that Sign produces a signature
// that can be verified with the public key.
func TestSignProducesVerifiableSignature(t *testing.T) {
	dir := t.TempDir()
	id, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}

	nonce := "test-challenge-nonce-42"
	sig64, signedAt, err := id.Sign(nonce)
	if err != nil {
		t.Fatalf("Sign() error: %v", err)
	}

	if signedAt <= 0 {
		t.Errorf("signedAt should be positive, got %d", signedAt)
	}

	sigDER, err := base64.StdEncoding.DecodeString(sig64)
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}

	h := sha256.Sum256([]byte(nonce))
	if !ecdsa.VerifyASN1(&id.key.PublicKey, h[:], sigDER) {
		t.Fatal("signature verification failed")
	}
}

// TestSignDifferentNoncesProduceDifferentSignatures verifies that signing
// different nonces produces different signatures.
func TestSignDifferentNoncesProduceDifferentSignatures(t *testing.T) {
	dir := t.TempDir()
	id, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}

	sig1, _, _ := id.Sign("nonce-1")
	sig2, _, _ := id.Sign("nonce-2")

	if sig1 == sig2 {
		t.Error("different nonces produced identical signatures")
	}
}

// TestLoadOrCreateCreatesDirectory verifies that LoadOrCreate creates the
// state directory if it does not exist.
func TestLoadOrCreateCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "path")

	_, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("LoadOrCreate with nested dir: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}
}

// TestLoadOrCreateRejectsCorruptKey verifies that a corrupt key file is
// reported as an error.
func TestLoadOrCreateRejectsCorruptKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, keyFile)
	if err := os.WriteFile(path, []byte("not a pem file"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadOrCreate(dir)
	if err == nil {
		t.Fatal("expected error for corrupt key file")
	}
}

// TestLoadOrCreateP256Curve verifies the generated key uses P-256.
func TestLoadOrCreateP256Curve(t *testing.T) {
	dir := t.TempDir()
	id, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}

	if id.key.Curve != elliptic.P256() {
		t.Errorf("expected P-256 curve, got %v", id.key.Curve.Params().Name)
	}
}
