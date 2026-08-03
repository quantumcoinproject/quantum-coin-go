// Copyright 2015 The go-ethereum Authors
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

package nat

import (
	"encoding/binary"
	"net"
	"sync"
	"testing"
	"time"

	natpmp "github.com/jackpal/go-nat-pmp"
)

// natPMPPort is the fixed port the NAT-PMP client talks to. It is not exported by
// go-nat-pmp, so the fake gateway below has to bind it.
const natPMPPort = 5351

// TestUBF103_NATPMPRejectsAlternatePort runs AddMapping against a fake gateway that
// grants a different external port than the one requested. Upstream 42212808f: the
// response used to be discarded, so the caller believed the requested port was mapped
// and advertised an unreachable endpoint.
func TestUBF103_NATPMPRejectsAlternatePort(t *testing.T) {
	const (
		requestedExt = 30303
		internal     = 30303
		grantedExt   = 40404
	)

	gwIP := net.IPv4(127, 0, 0, 1)
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: gwIP, Port: natPMPPort})
	if err != nil {
		t.Skipf("cannot bind the NAT-PMP port on loopback: %v", err)
	}
	defer conn.Close()

	var (
		mu       sync.Mutex
		requests [][]byte
	)
	go func() {
		buf := make([]byte, 64)
		for {
			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			req := append([]byte(nil), buf[:n]...)
			mu.Lock()
			requests = append(requests, req)
			mu.Unlock()

			// Successful AddPortMapping response, but with a different external port.
			res := make([]byte, 16)
			res[0] = 0             // version
			res[1] = req[1] | 0x80 // response opcode
			// bytes 2:4 stay zero: result code "success"
			copy(res[8:10], req[4:6]) // internal port, echoed
			binary.BigEndian.PutUint16(res[10:12], grantedExt)
			copy(res[12:16], req[8:12]) // lifetime, echoed
			conn.WriteToUDP(res, addr)
		}
	}()

	n := &pmp{gw: gwIP, c: natpmp.NewClientWithTimeout(gwIP, 5*time.Second)}
	if err := n.AddMapping("TCP", requestedExt, internal, "test", 10*time.Minute); err == nil {
		t.Fatal("AddMapping succeeded even though the gateway mapped a different external port")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requests) < 2 {
		t.Fatalf("expected the bogus mapping to be torn down, got %d request(s)", len(requests))
	}
	teardown := requests[1]
	if got := binary.BigEndian.Uint16(teardown[6:8]); got != 0 {
		t.Errorf("teardown requested external port %d, want 0", got)
	}
	if got := binary.BigEndian.Uint32(teardown[8:12]); got != 0 {
		t.Errorf("teardown requested lifetime %d, want 0", got)
	}
}
