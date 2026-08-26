package security

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost controls how expensive password hashing is. bcrypt.DefaultCost (=10)
// is recommended for 2020+ hardware and is what the standard library defaults to.
// We deliberately do NOT use bcrypt.MinCost (=4), which finishes in milliseconds
// and would be trivially brute-forceable from a stolen hash.
const bcryptCost = bcrypt.DefaultCost

// GeneratePassword returns a bcrypt hash of the plaintext password using
// bcryptCost. If hashing fails (extremely unlikely — only on exhausted entropy
// or a bad cost constant), it returns the error string so the caller can store
// it in place of the hash. Callers should still check the result is a valid
// bcrypt-prefixed hash before persisting.
func GeneratePassword(p string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(p), bcryptCost)
	if err != nil {
		return err.Error()
	}
	return string(hash)
}

// ComparePasswords checks if the plaintext password matches the stored hash.
func ComparePasswords(hashedPwd, inputPwd string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashedPwd), []byte(inputPwd)) == nil
}

// NewToken returns an unpredictable token derived from the input. It is used
// to materialize non-password secrets (e.g. JWT signing keys bootstrapped
// during install).
//
// Construction: PBKDF2-HMAC-SHA512(input, random salt, 4096, 32 bytes), then
// encode as hex(salt):hex(key) so the result is self-contained.
func NewToken(text string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("salt generation: %w", err)
	}

	key, err := pbkdf2.Key(sha512.New, text, salt, 4096, 32)
	if err != nil {
		return "", fmt.Errorf("pbkdf2 derive: %w", err)
	}

	return hex.EncodeToString(key), nil
}
