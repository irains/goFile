package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

var activeStateLocks = struct {
	sync.Mutex
	paths map[string]struct{}
}{paths: make(map[string]struct{})}

// stateLock owns the state lock file. Closing its file descriptor releases the
// operating-system lock on every supported platform.
type stateLock struct {
	file       *os.File
	registryID string
}

func openStateLockFile(path string) (*stateLock, error) {
	registryID, err := filepath.Abs(path)
	if err != nil {
		return nil, errors.New("could not resolve state lock")
	}
	registryID = filepath.Clean(registryID)
	activeStateLocks.Lock()
	if _, held := activeStateLocks.paths[registryID]; held {
		activeStateLocks.Unlock()
		return nil, errors.New("state directory is already in use")
	}
	activeStateLocks.paths[registryID] = struct{}{}
	activeStateLocks.Unlock()
	releaseRegistry := true
	defer func() {
		if releaseRegistry {
			activeStateLocks.Lock()
			delete(activeStateLocks.paths, registryID)
			activeStateLocks.Unlock()
		}
	}()
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, errors.New("state lock must be a regular file")
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, errors.New("could not inspect state lock")
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, errors.New("could not open state lock")
	}
	if err := protectPrivateFile(path); err != nil {
		_ = file.Close()
		return nil, errors.New("could not protect state lock")
	}
	releaseRegistry = false
	return &stateLock{file: file, registryID: registryID}, nil
}

func (lock *stateLock) Close() error {
	if lock == nil || lock.file == nil {
		return nil
	}
	file := lock.file
	registryID := lock.registryID
	lock.file = nil
	lock.registryID = ""
	err := file.Close()
	if registryID != "" {
		activeStateLocks.Lock()
		delete(activeStateLocks.paths, registryID)
		activeStateLocks.Unlock()
	}
	return err
}
