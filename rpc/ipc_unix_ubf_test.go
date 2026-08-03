// Copyright 2026 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

//go:build darwin || dragonfly || freebsd || linux || nacl || netbsd || openbsd || solaris
// +build darwin dragonfly freebsd linux nacl netbsd openbsd solaris

package rpc

import (
	"strings"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/log"
)

// TestUBF070_IPCPathLength checks that the endpoint length check accounts for the
// null terminator: a path of exactly max_path_size characters does not fit in
// sockaddr_un.sun_path and must be warned about. Upstream 8e92881a3 (#26614).
func TestUBF070_IPCPathLength(t *testing.T) {
	var warned bool
	old := log.Root().GetHandler()
	log.Root().SetHandler(log.FuncHandler(func(r *log.Record) error {
		if strings.Contains(r.Msg, "ipc endpoint is longer") {
			warned = true
		}
		return nil
	}))
	defer log.Root().SetHandler(old)

	// A path of exactly max_path_size bytes needs max_path_size+1 with its terminator,
	// so it must be reported. The listen itself fails; only the warning matters here.
	endpoint := "/tmp/" + strings.Repeat("a", int(max_path_size)-5)
	if len(endpoint) != int(max_path_size) {
		t.Fatalf("test endpoint has length %d, want %d", len(endpoint), max_path_size)
	}
	l, err := ipcListen(endpoint)
	if err == nil {
		l.Close()
	}
	if !warned {
		t.Fatal("no warning for an endpoint that does not fit including its null terminator")
	}
}
