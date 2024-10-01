// Copyright 2015 Matthew Holt and The Kengine Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logging

import (
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/kengineconfig/kenginefile"
)

func init() {
	kengine.RegisterModule(NetWriter{})
}

// NetWriter implements a log writer that outputs to a network socket. If
// the socket goes down, it will dump logs to stderr while it attempts to
// reconnect.
type NetWriter struct {
	// The address of the network socket to which to connect.
	Address string `json:"address,omitempty"`

	// The timeout to wait while connecting to the socket.
	DialTimeout kengine.Duration `json:"dial_timeout,omitempty"`

	// If enabled, allow connections errors when first opening the
	// writer. The error and subsequent log entries will be reported
	// to stderr instead until a connection can be re-established.
	SoftStart bool `json:"soft_start,omitempty"`

	addr kengine.NetworkAddress
}

// KengineModule returns the Kengine module information.
func (NetWriter) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "kengine.logging.writers.net",
		New: func() kengine.Module { return new(NetWriter) },
	}
}

// Provision sets up the module.
func (nw *NetWriter) Provision(ctx kengine.Context) error {
	repl := kengine.NewReplacer()
	address, err := repl.ReplaceOrErr(nw.Address, true, true)
	if err != nil {
		return fmt.Errorf("invalid host in address: %v", err)
	}

	nw.addr, err = kengine.ParseNetworkAddress(address)
	if err != nil {
		return fmt.Errorf("parsing network address '%s': %v", address, err)
	}

	if nw.addr.PortRangeSize() != 1 {
		return fmt.Errorf("multiple ports not supported")
	}

	if nw.DialTimeout < 0 {
		return fmt.Errorf("timeout cannot be less than 0")
	}

	return nil
}

func (nw NetWriter) String() string {
	return nw.addr.String()
}

// WriterKey returns a unique key representing this nw.
func (nw NetWriter) WriterKey() string {
	return nw.addr.String()
}

// OpenWriter opens a new network connection.
func (nw NetWriter) OpenWriter() (io.WriteCloser, error) {
	reconn := &redialerConn{
		nw:      nw,
		timeout: time.Duration(nw.DialTimeout),
	}
	conn, err := reconn.dial()
	if err != nil {
		if !nw.SoftStart {
			return nil, err
		}
		// don't block config load if remote is down or some other external problem;
		// we can dump logs to stderr for now (see issue #5520)
		fmt.Fprintf(os.Stderr, "[ERROR] net log writer failed to connect: %v (will retry connection and print errors here in the meantime)\n", err)
	}
	reconn.connMu.Lock()
	reconn.Conn = conn
	reconn.connMu.Unlock()
	return reconn, nil
}

// UnmarshalKenginefile sets up the handler from Kenginefile tokens. Syntax:
//
//	net <address> {
//	    dial_timeout <duration>
//	    soft_start
//	}
func (nw *NetWriter) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	d.Next() // consume writer name
	if !d.NextArg() {
		return d.ArgErr()
	}
	nw.Address = d.Val()
	if d.NextArg() {
		return d.ArgErr()
	}
	for d.NextBlock(0) {
		switch d.Val() {
		case "dial_timeout":
			if !d.NextArg() {
				return d.ArgErr()
			}
			timeout, err := kengine.ParseDuration(d.Val())
			if err != nil {
				return d.Errf("invalid duration: %s", d.Val())
			}
			if d.NextArg() {
				return d.ArgErr()
			}
			nw.DialTimeout = kengine.Duration(timeout)

		case "soft_start":
			if d.NextArg() {
				return d.ArgErr()
			}
			nw.SoftStart = true
		}
	}
	return nil
}

// redialerConn wraps an underlying Conn so that if any
// writes fail, the connection is redialed and the write
// is retried.
type redialerConn struct {
	net.Conn
	connMu     sync.RWMutex
	nw         NetWriter
	timeout    time.Duration
	lastRedial time.Time
}

// Write wraps the underlying Conn.Write method, but if that fails,
// it will re-dial the connection anew and try writing again.
func (reconn *redialerConn) Write(b []byte) (n int, err error) {
	reconn.connMu.RLock()
	conn := reconn.Conn
	reconn.connMu.RUnlock()
	if conn != nil {
		if n, err = conn.Write(b); err == nil {
			return
		}
	}

	// problem with the connection - lock it and try to fix it
	reconn.connMu.Lock()
	defer reconn.connMu.Unlock()

	// if multiple concurrent writes failed on the same broken conn, then
	// one of them might have already re-dialed by now; try writing again
	if reconn.Conn != nil {
		if n, err = reconn.Conn.Write(b); err == nil {
			return
		}
	}

	// there's still a problem, so try to re-attempt dialing the socket
	// if some time has passed in which the issue could have potentially
	// been resolved - we don't want to block at every single log
	// emission (!) - see discussion in #4111
	if time.Since(reconn.lastRedial) > 10*time.Second {
		reconn.lastRedial = time.Now()
		conn2, err2 := reconn.dial()
		if err2 != nil {
			// logger socket still offline; instead of discarding the log, dump it to stderr
			os.Stderr.Write(b)
			return
		}
		if n, err = conn2.Write(b); err == nil {
			if reconn.Conn != nil {
				reconn.Conn.Close()
			}
			reconn.Conn = conn2
		}
	} else {
		// last redial attempt was too recent; just dump to stderr for now
		os.Stderr.Write(b)
	}

	return
}

func (reconn *redialerConn) dial() (net.Conn, error) {
	return net.DialTimeout(reconn.nw.addr.Network, reconn.nw.addr.JoinHostPort(0), reconn.timeout)
}

// Interface guards
var (
	_ kengine.Provisioner     = (*NetWriter)(nil)
	_ kengine.WriterOpener    = (*NetWriter)(nil)
	_ kenginefile.Unmarshaler = (*NetWriter)(nil)
)
