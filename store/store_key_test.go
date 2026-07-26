package store

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestAPIKeyGenerationAndHashing(t *testing.T) {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		t.Fatalf("Failed to generate random bytes: %v", err)
	}

	rawKey := "ce_" + base64.RawURLEncoding.EncodeToString(bytes)
	if len(rawKey) < 8 {
		t.Fatalf("Generated key too short: %s", rawKey)
	}

	prefix := rawKey[:8]
	if prefix[:3] != "ce_" {
		t.Errorf("Expected prefix to start with 'ce_', got %s", prefix)
	}

	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	if len(keyHash) != 64 {
		t.Errorf("Expected SHA-256 hex hash length of 64, got %d", len(keyHash))
	}
}
