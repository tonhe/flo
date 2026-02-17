package identity

import (
	"bytes"
	"testing"
)

func TestDeriveKey(t *testing.T) {
	salt := make([]byte, 16)
	key := DeriveKey([]byte("test-password"), salt)
	if len(key) != 32 {
		t.Errorf("expected 32 byte key, got %d", len(key))
	}
}

func TestDeriveKeyDeterministic(t *testing.T) {
	salt := []byte("fixed-salt-value")
	key1 := DeriveKey([]byte("password"), salt)
	key2 := DeriveKey([]byte("password"), salt)
	if !bytes.Equal(key1, key2) {
		t.Error("same password+salt should produce same key")
	}
}

func TestDeriveKeyDifferentPasswords(t *testing.T) {
	salt := []byte("fixed-salt-value")
	key1 := DeriveKey([]byte("password1"), salt)
	key2 := DeriveKey([]byte("password2"), salt)
	if bytes.Equal(key1, key2) {
		t.Error("different passwords should produce different keys")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	key := make([]byte, 32)
	plaintext := []byte("secret data for testing")

	ciphertext, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}
	if bytes.Equal(ciphertext, plaintext) {
		t.Error("ciphertext should differ from plaintext")
	}

	decrypted, err := Decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted data doesn't match: got %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	key2[0] = 1 // different key

	ciphertext, err := Encrypt(key1, []byte("secret"))
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}

	_, err = Decrypt(key2, ciphertext)
	if err == nil {
		t.Error("Decrypt with wrong key should fail")
	}
}

func TestGenerateSalt(t *testing.T) {
	salt, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt() error: %v", err)
	}
	if len(salt) != 16 {
		t.Errorf("expected 16 byte salt, got %d", len(salt))
	}
}
