package main

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"github.com/irains/fileharbor/auth"
	"io"
	"os"

	"golang.org/x/term"
)

type passwordPrompter interface {
	IsTerminal() bool
	ReadPassword() ([]byte, error)
}

type terminalPasswordPrompter struct {
	file *os.File
}

func (p terminalPasswordPrompter) IsTerminal() bool {
	return term.IsTerminal(int(p.file.Fd()))
}

func (p terminalPasswordPrompter) ReadPassword() ([]byte, error) {
	return term.ReadPassword(int(p.file.Fd()))
}

// runHashPasswordCommand interactively creates an administrator bcrypt hash.
// Prompts and diagnostics use stderr so stdout is safe to capture as a secret.
func runHashPasswordCommand(args []string, prompt passwordPrompter, stdout, stderr io.Writer) error {
	if len(args) != 0 {
		return errors.New("hash-password does not accept arguments")
	}
	if prompt == nil || !prompt.IsTerminal() {
		return errors.New("hash-password requires an interactive terminal")
	}

	if _, err := fmt.Fprint(stderr, "Password: "); err != nil {
		return err
	}
	password, err := prompt.ReadPassword()
	if err != nil {
		return errors.New("could not read password")
	}
	defer auth.ClearPassword(password)
	if _, err := fmt.Fprintln(stderr); err != nil {
		return err
	}

	if _, err := fmt.Fprint(stderr, "Confirm password: "); err != nil {
		return err
	}
	confirmation, err := prompt.ReadPassword()
	if err != nil {
		return errors.New("could not read password confirmation")
	}
	defer auth.ClearPassword(confirmation)
	if _, err := fmt.Fprintln(stderr); err != nil {
		return err
	}

	if err := auth.ValidatePassword(password); err != nil {
		if errors.Is(err, auth.ErrPasswordTooLong) {
			return errors.New("password must contain at most 72 bytes")
		}
		return errors.New("password must not be empty")
	}
	if subtle.ConstantTimeCompare(password, confirmation) != 1 {
		return errors.New("password confirmation does not match")
	}
	hash, err := auth.GeneratePasswordHash(password)
	if err != nil {
		return errors.New("could not generate password hash")
	}
	_, err = fmt.Fprintln(stdout, hash)
	return err
}
