// Command generate_bcrypt_hash prints a bcrypt hash for CI-only input supplied via environment.
package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	password := os.Getenv("FILEHARBOR_TEST_PASSWORD")
	if password == "" {
		fmt.Fprintln(os.Stderr, "FILEHARBOR_TEST_PASSWORD is required")
		os.Exit(2)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not generate bcrypt hash")
		os.Exit(1)
	}
	fmt.Print(string(hash))
}
