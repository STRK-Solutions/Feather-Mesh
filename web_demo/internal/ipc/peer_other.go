//go:build !linux && !darwin

package ipc

import (
	"errors"
	"net"
)

func peerUID(net.Conn) (uint32, error) { return 0, errors.New("unsupported peer credentials") }
