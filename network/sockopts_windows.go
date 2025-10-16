//go:build windows
// +build windows

package network

import "syscall"

// setSocketReuseAddrPort sets socket reuse options for Windows.
// Note: Windows implementation is currently a stub as SO_REUSEADDR/SO_REUSEPORT
// have different semantics on Windows compared to Unix systems.
// This function accepts the conn parameter to maintain API compatibility.
func setSocketReuseAddrPort(_ syscall.RawConn) error {
	return nil
}
