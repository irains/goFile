//go:build !linux && !darwin && !windows

package utils

import "errors"

func renameNoReplace(source, destination string) error {
	return errors.New("atomic no-replace rename unsupported")
}
