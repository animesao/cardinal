//go:build !linux

package builder

import "os"

func openCopySource(path string) (*os.File, error) {
	return os.Open(path)
}

func openCopyDestination(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
}
