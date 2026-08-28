package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

const (
	// PasswordHashCost is the bcrypt cost used by the password-hash command.
	PasswordHashCost    = bcrypt.DefaultCost
	maxPasswordHashCost = 12
	maxPasswordBytes    = 72
)

var (
	ErrInvalidPassword  = errors.New("invalid password")
	ErrPasswordTooLong  = errors.New("password exceeds bcrypt byte limit")
)

// ValidatePassword enforces the plaintext password constraints shared by the
// interactive generator and authentication checks.
func ValidatePassword(password []byte) error {
	if len(password) == 0 {
		return ErrInvalidPassword
	}
	if len(password) > maxPasswordBytes {
		return ErrPasswordTooLong
	}
	return nil
}

// GeneratePasswordHash creates a salted bcrypt verifier for an administrator
// password. The plaintext is never retained by this package after the call.
func GeneratePasswordHash(password []byte) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword(password, PasswordHashCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func validPasswordHash(hash string) bool {
	cost, err := bcrypt.Cost([]byte(hash))
	return err == nil && cost >= PasswordHashCost && cost <= maxPasswordHashCost
}

// ClearPassword overwrites a mutable password buffer as soon as it is no longer
// needed. It cannot erase immutable string copies held by the Go runtime.
func ClearPassword(password []byte) {
	for i := range password {
		password[i] = 0
	}
}

func passwordForComparison(password []byte) ([]byte, bool) {
	if ValidatePassword(password) != nil {
		return []byte{0}, false
	}
	return password, true
}
