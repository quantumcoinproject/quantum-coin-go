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

package ethstats

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

// TestUBF057_EthstatsReadLimit checks that the connection to the stats server has a read
// limit, so a malicious server cannot exhaust our memory with one giant frame.
// Upstream c2e0abce2 (#26207).
func TestUBF057_EthstatsReadLimit(t *testing.T) {
	if messageSizeLimit != 15*1024*1024 {
		t.Fatalf("messageSizeLimit = %d, want 15 MiB", messageSizeLimit)
	}

	var (
		upgrader = websocket.Upgrader{}
		serverCh = make(chan *websocket.Conn, 1)
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error("upgrade failed:", err)
			return
		}
		serverCh <- conn
	}))
	defer srv.Close()

	wsURL := "ws:" + strings.TrimPrefix(srv.URL, "http:")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal("dial failed:", err)
	}
	defer clientConn.Close()

	// This is the wrapper ethstats uses for the connection to the stats server.
	wrapped := newConnectionWrapper(clientConn)
	defer wrapped.Close()

	// The stats server sends a frame larger than the limit.
	serverConn := <-serverCh
	defer serverConn.Close()
	go func() {
		serverConn.WriteMessage(websocket.TextMessage, make([]byte, messageSizeLimit+1024))
	}()

	var v interface{}
	err = wrapped.ReadJSON(&v)
	if err == nil {
		t.Fatal("oversized message was accepted; no read limit is set")
	}
	if !websocket.IsCloseError(err, websocket.CloseMessageTooBig) &&
		!strings.Contains(err.Error(), "read limit exceeded") {
		t.Fatalf("want a read-limit error, got %v", err)
	}
}
