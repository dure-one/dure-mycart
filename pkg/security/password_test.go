package security

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestGeneratePassword_ProducesValidBcrypt(t *testing.T) {
	t.Parallel()

	const plain = "Sup3rSecret!"
	hash := GeneratePassword(plain)
	if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
		t.Fatalf("hash does not look like bcrypt: %q", hash)
	}

	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		t.Fatalf("bcrypt.Cost: %v", err)
	}
	if cost < bcrypt.DefaultCost {
		t.Errorf("cost %d is below DefaultCost %d — weak hash", cost, bcrypt.DefaultCost)
	}
	if !ComparePasswords(hash, plain) {
		t.Error("ComparePasswords returned false for the matching password")
	}
	if ComparePasswords(hash, plain+"x") {
		t.Error("ComparePasswords returned true for a wrong password")
	}
}

func TestNewToken_IsHexAndStable(t *testing.T) {
	t.Parallel()

	tok, err := NewToken("seed-value")
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	// SHA-256 hex = 64 chars.
	if len(tok) != 64 {
		t.Fatalf("token length = %d, want 64", len(tok))
	}
	for _, r := range tok {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			t.Fatalf("token contains non-hex char %q", r)
		}
	}
	// Different inputs must yield different outputs.
	other, err := NewToken("other-seed")
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	if tok == other {
		t.Error("NewToken returned identical outputs for different inputs")
	}
}

// bcrypt refuses inputs longer than 72 bytes. GeneratePassword must surface that
// as the error string rather than a truncated hash of the first 72 bytes: a
// silent truncation would make two different passwords share one hash.
func TestGeneratePassword_TooLongInput(t *testing.T) {
	t.Parallel()

	const tooLong = 73

	got := GeneratePassword(strings.Repeat("a", tooLong))
	if got == "" {
		t.Fatal("GeneratePassword returned an empty string")
	}
	if !strings.HasPrefix(got, "bcrypt:") {
		t.Errorf("an over-long password should produce bcrypt's error text, got %q", got)
	}
	if ComparePasswords(got, strings.Repeat("a", tooLong)) {
		t.Error("the error text was accepted as a password hash")
	}
}

func TestNewToken_TooLongInput(t *testing.T) {
	t.Parallel()

	token, err := NewToken(strings.Repeat("a", 73))
	if err == nil {
		t.Fatalf("an over-long token input was accepted, token = %q", token)
	}
	if token != "" {
		t.Errorf("token returned alongside the error: %q", token)
	}
	if !strings.Contains(err.Error(), "bcrypt hash:") {
		t.Errorf("error %q does not say where it came from", err)
	}
}
