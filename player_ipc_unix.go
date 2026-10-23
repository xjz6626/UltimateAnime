//go:build !windows

package main

import (
	"context"
	"io"
	"net"
	"path/filepath"
)

func playerIPCAddress(directory string) string { return filepath.Join(directory, "socket") }
func dialPlayerIPC(ctx context.Context, address string) (io.ReadWriteCloser, error) {
	return (&net.Dialer{}).DialContext(ctx, "unix", address)
}
