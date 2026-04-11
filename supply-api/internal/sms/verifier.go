package sms

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CodeEntry represents a stored verification code entry.
type CodeEntry struct {
	Code      string
	Phone     string
	ExpiresAt time.Time
	Used      bool
}

// SMSCodeVerifier is an SMS verifier that stores and verifies codes.
type SMSCodeVerifier struct {
	mu    sync.RWMutex
	codes map[string]*CodeEntry
	store *InMemoryCodeStore
}

// NewSMSCodeVerifier creates a new SMS code verifier with in-memory storage.
func NewSMSCodeVerifier() *SMSCodeVerifier {
	return &SMSCodeVerifier{
		codes: make(map[string]*CodeEntry),
		store: NewInMemoryCodeStore(),
	}
}

// SendVerificationCode sends a verification code and stores it.
func (v *SMSCodeVerifier) SendVerificationCode(ctx context.Context, phoneNumber string) (string, error) {
	code, err := GenerateCode(6)
	if err != nil {
		return "", fmt.Errorf("failed to generate code: %w", err)
	}

	codeID := fmt.Sprintf("verifier-%d", time.Now().UnixNano())
	expiresAt := time.Now().Add(5 * time.Minute)

	v.mu.Lock()
	v.codes[codeID] = &CodeEntry{
		Code:      code,
		Phone:     phoneNumber,
		ExpiresAt: expiresAt,
		Used:      false,
	}
	v.mu.Unlock()

	// In production, send SMS via provider
	fmt.Printf("[SMSCodeVerifier] Code '%s' for phone %s (ID: %s)\n", code, phoneNumber, codeID)
	return codeID, nil
}

// Verify verifies the code for the given phone number.
func (v *SMSCodeVerifier) Verify(ctx context.Context, phone string, code string) (bool, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Check all entries for matching phone and code
	for id, entry := range v.codes {
		if entry.Phone == phone && entry.Code == code && !entry.Used {
			if time.Now().After(entry.ExpiresAt) {
				delete(v.codes, id)
				return false, nil
			}
			entry.Used = true
			delete(v.codes, id)
			return true, nil
		}
	}

	return false, nil
}

// VerifyByID verifies the code by code ID.
func (v *SMSCodeVerifier) VerifyByID(ctx context.Context, codeID string, phone string, code string) (bool, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	entry, ok := v.codes[codeID]
	if !ok {
		return false, nil
	}

	if time.Now().After(entry.ExpiresAt) {
		delete(v.codes, codeID)
		return false, nil
	}

	if entry.Phone != phone || entry.Code != code || entry.Used {
		return false, nil
	}

	entry.Used = true
	delete(v.codes, codeID)
	return true, nil
}

// Cleanup removes expired codes.
func (v *SMSCodeVerifier) Cleanup() {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now()
	for id, entry := range v.codes {
		if now.After(entry.ExpiresAt) {
			delete(v.codes, id)
		}
	}
}
