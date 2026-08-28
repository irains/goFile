package utils

import "errors"

func isCrossDeviceError(err error) bool {
	return errors.Is(err, errCrossDevice)
}
