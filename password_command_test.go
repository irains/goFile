package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

type testPasswordPrompter struct {
	terminal bool
	values   [][]byte
	errors   []error
	err      error
	reads    int
}

func (p *testPasswordPrompter) IsTerminal() bool { return p.terminal }

func (p *testPasswordPrompter) ReadPassword() ([]byte, error) {
	p.reads++
	if len(p.errors) != 0 {
		err := p.errors[0]
		p.errors = p.errors[1:]
		if err != nil {
			return nil, err
		}
	}
	if p.err != nil {
		return nil, p.err
	}
	if len(p.values) == 0 {
		return nil, errors.New("unexpected password read")
	}
	value := p.values[0]
	p.values = p.values[1:]
	return value, nil
}

func TestHashPasswordCommandRejectsUnsafeInput(t *testing.T) {
	for _, test := range []struct {
		name   string
		args   []string
		prompt *testPasswordPrompter
	}{
		{"non-terminal", nil, &testPasswordPrompter{}},
		{"extra argument", []string{"password"}, &testPasswordPrompter{terminal: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if err := runHashPasswordCommand(test.args, test.prompt, &stdout, &stderr); err == nil {
				t.Fatal("command should fail")
			}
			if stdout.Len() != 0 || test.prompt.reads != 0 {
				t.Fatalf("unsafe input wrote output or prompted: stdout=%q reads=%d", stdout.String(), test.prompt.reads)
			}
		})
	}
}

func TestHashPasswordCommandValidatesInputAndClearsBuffers(t *testing.T) {
	for _, test := range []struct {
		name   string
		values [][]byte
	}{
		{"empty", [][]byte{[]byte{}, []byte{}}},
		{"mismatch", [][]byte{[]byte("first"), []byte("second")}},
		{"too long", [][]byte{[]byte(strings.Repeat("x", 73)), []byte(strings.Repeat("x", 73))}},
	} {
		t.Run(test.name, func(t *testing.T) {
			password, confirmation := test.values[0], test.values[1]
			prompt := &testPasswordPrompter{terminal: true, values: test.values}
			var stdout, stderr bytes.Buffer
			if err := runHashPasswordCommand(nil, prompt, &stdout, &stderr); err == nil {
				t.Fatal("command should fail")
			}
			if stdout.Len() != 0 {
				t.Fatalf("command wrote hash on failure: %q", stdout.String())
			}
			for _, value := range [][]byte{password, confirmation} {
				for _, b := range value {
					if b != 0 {
						t.Fatal("password buffer was not cleared")
					}
				}
			}
		})
	}
}

func TestHashPasswordCommandWritesOnlyHash(t *testing.T) {
	password := []byte("a durable password")
	confirmation := []byte("a durable password")
	prompt := &testPasswordPrompter{terminal: true, values: [][]byte{password, confirmation}}
	var stdout, stderr bytes.Buffer
	if err := runHashPasswordCommand(nil, prompt, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	hash := strings.TrimSuffix(stdout.String(), "\n")
	if stdout.String() != hash+"\n" || bcrypt.CompareHashAndPassword([]byte(hash), []byte("a durable password")) != nil {
		t.Fatalf("unexpected command output: %q", stdout.String())
	}
	if strings.Contains(stderr.String(), "a durable password") || strings.Contains(stderr.String(), hash) {
		t.Fatal("password or hash leaked to stderr")
	}
	for _, value := range [][]byte{password, confirmation} {
		for _, b := range value {
			if b != 0 {
				t.Fatal("password buffer was not cleared")
			}
		}
	}
}

func TestHashPasswordCommandReadError(t *testing.T) {
	prompt := &testPasswordPrompter{terminal: true, err: errors.New("terminal failure")}
	var stdout, stderr bytes.Buffer
	if err := runHashPasswordCommand(nil, prompt, &stdout, &stderr); err == nil {
		t.Fatal("read error should fail")
	}
	if stdout.Len() != 0 || strings.Contains(stderr.String(), "terminal failure") {
		t.Fatal("command exposed unsafe output")
	}
}

func TestHashPasswordCommandClearsPasswordAfterConfirmationReadError(t *testing.T) {
	password := []byte("a durable password")
	prompt := &testPasswordPrompter{
		terminal: true,
		values:   [][]byte{password},
		errors:   []error{nil, errors.New("terminal failure")},
	}
	var stdout, stderr bytes.Buffer
	if err := runHashPasswordCommand(nil, prompt, &stdout, &stderr); err == nil {
		t.Fatal("confirmation read error should fail")
	}
	if stdout.Len() != 0 || strings.Contains(stderr.String(), "terminal failure") {
		t.Fatal("command exposed unsafe output")
	}
	for _, b := range password {
		if b != 0 {
			t.Fatal("password buffer was not cleared after confirmation read failure")
		}
	}
}
