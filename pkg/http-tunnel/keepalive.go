// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

// +build !windows

package tunnel

import (
	"net"
	"time"

	"github.com/felixge/tcpkeepalive"
)

var (
	// DefaultKeepAliveIdleTime specifies how long connection can be idle
	// before sending keepalive message.
	DefaultKeepAliveIdleTime = 600 * time.Second
	// DefaultKeepAliveCount specifies maximal number of keepalive messages
	// sent before marking connection as dead.
	DefaultKeepAliveCount = 20
	// DefaultKeepAliveInterval specifies how often retry sending keepalive
	// messages when no response is received.
	DefaultKeepAliveInterval = 60 * time.Second
)

func KeepAlive(conn net.Conn) error {
	return tcpkeepalive.SetKeepAlive(conn, DefaultKeepAliveIdleTime, DefaultKeepAliveCount, DefaultKeepAliveInterval)
}
