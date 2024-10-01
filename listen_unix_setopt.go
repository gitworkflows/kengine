//go:build unix && !freebsd && !solaris

package kengine

import "golang.org/x/sys/unix"

const unixSOREUSEPORT = unix.SO_REUSEPORT
