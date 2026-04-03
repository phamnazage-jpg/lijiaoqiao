package config

import (
	"testing"
)

func TestMED03_DatabasePassword_GetPasswordReturnsDecrypted(t *testing.T) {
	// MED-03: Database password should be encrypted when stored
	// GetPassword() method should return decrypted password

	// Test with EncryptedPassword field
	cfg := &DatabaseConfig{
		Host:              "localhost",
		Port:              5432,
		User:              "postgres",
		EncryptedPassword: "dGVzdDEyMw==", // base64 encoded "test123" in AES-GCM format
		Database:          "gateway",
		MaxConns:          10,
	}

	// After fix: GetPassword() should return decrypted value
	password := cfg.GetPassword()
	if password == "" {
		t.Error("GetPassword should return non-empty decrypted password")
	}
}

func TestMED03_EncryptedPasswordField(t *testing.T) {
	// Test that encrypted password can be properly encrypted and decrypted
	originalPassword := "mysecretpassword123"

	// Encrypt the password
	encrypted, err := encryptPassword(originalPassword)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	if encrypted == "" {
		t.Error("encryption should produce non-empty result")
	}

	// Encrypted password should be different from original
	if encrypted == originalPassword {
		t.Error("encrypted password should differ from original")
	}

	// Should be able to decrypt back to original
	decrypted, err := decryptPassword(encrypted)
	if err != nil {
		t.Fatalf("decryption failed: %v", err)
	}
	if decrypted != originalPassword {
		t.Errorf("decrypted password should match original, got %s", decrypted)
	}
}

func TestMED03_PasswordGetterReturnsDecrypted(t *testing.T) {
	// Test that GetPassword returns decrypted password
	originalPassword := "production_secret_456"
	encrypted, err := encryptPassword(originalPassword)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	cfg := &DatabaseConfig{
		Host:              "localhost",
		Port:              5432,
		User:              "postgres",
		EncryptedPassword: encrypted,
		Database:          "gateway",
		MaxConns:          10,
	}

	// After fix: GetPassword() should return decrypted value
	password := cfg.GetPassword()
	if password != originalPassword {
		t.Errorf("GetPassword should return decrypted password, got %s", password)
	}
}

func TestMED03_FallbackToPlainPassword(t *testing.T) {
	// Test that if EncryptedPassword is empty, Password field is used
	cfg := &DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "fallback_password",
		Database: "gateway",
		MaxConns: 10,
	}

	password := cfg.GetPassword()
	if password != "fallback_password" {
		t.Errorf("GetPassword should fallback to Password field, got %s", password)
	}
}

func TestMED03_RedisPassword_GetPasswordReturnsDecrypted(t *testing.T) {
	// Test Redis password encryption as well
	originalPassword := "redis_secret_pass"
	encrypted, err := encryptPassword(originalPassword)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	cfg := &RedisConfig{
		Host:              "localhost",
		Port:              6379,
		EncryptedPassword: encrypted,
		DB:                0,
		PoolSize:          10,
	}

	password := cfg.GetPassword()
	if password != originalPassword {
		t.Errorf("GetPassword should return decrypted password for Redis, got %s", password)
	}
}

func TestMED03_EncryptEmptyString(t *testing.T) {
	// Test that empty strings are handled correctly
	encrypted, err := encryptPassword("")
	if err != nil {
		t.Fatalf("encryption of empty string failed: %v", err)
	}
	if encrypted != "" {
		t.Error("encryption of empty string should return empty string")
	}

	decrypted, err := decryptPassword("")
	if err != nil {
		t.Fatalf("decryption of empty string failed: %v", err)
	}
	if decrypted != "" {
		t.Error("decryption of empty string should return empty string")
	}
}