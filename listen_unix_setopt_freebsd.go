//go:build freebsd

package kengine

import "golang.org/x/sys/unix"

const unixSOREUSEPORT = unix.SO_REUSEPORT_LB
