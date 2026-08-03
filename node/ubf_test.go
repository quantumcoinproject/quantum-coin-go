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

package node

import (
	"net/http"
	"testing"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/internal/testlog"
	"github.com/quantumcoinproject/quantum-coin-go/log"
	"github.com/quantumcoinproject/quantum-coin-go/rpc"
)

// TestUBF056_ReadHeaderTimeoutSet checks that ReadHeaderTimeout is sanitized and wired
// into both HTTP server entry points, closing the slowloris hole.
// Upstream 9244f87dc (#25338).
func TestUBF056_ReadHeaderTimeoutSet(t *testing.T) {
	// CheckTimeouts must fill in a sane value.
	timeouts := rpc.HTTPTimeouts{}
	CheckTimeouts(&timeouts)
	if timeouts.ReadHeaderTimeout != rpc.DefaultHTTPTimeouts.ReadHeaderTimeout {
		t.Fatalf("CheckTimeouts left ReadHeaderTimeout = %v", timeouts.ReadHeaderTimeout)
	}

	// StartHTTPEndpoint must set it on the http.Server.
	srv, _, err := StartHTTPEndpoint("127.0.0.1:0", rpc.HTTPTimeouts{}, http.NotFoundHandler())
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	if srv.ReadHeaderTimeout != rpc.DefaultHTTPTimeouts.ReadHeaderTimeout {
		t.Fatalf("StartHTTPEndpoint: ReadHeaderTimeout = %v, want %v",
			srv.ReadHeaderTimeout, rpc.DefaultHTTPTimeouts.ReadHeaderTimeout)
	}

	// httpServer.start must set it too.
	h := createAndStartServer(t, &httpConfig{}, false, &wsConfig{})
	defer h.stop()
	if h.server.ReadHeaderTimeout != rpc.DefaultHTTPTimeouts.ReadHeaderTimeout {
		t.Fatalf("httpServer.start: ReadHeaderTimeout = %v, want %v",
			h.server.ReadHeaderTimeout, rpc.DefaultHTTPTimeouts.ReadHeaderTimeout)
	}
}

// TestUBF074_ShutdownTimeout checks that doStop gives up on the graceful shutdown after
// shutdownTimeout instead of hanging on a busy connection, and that a shutdown which
// completes normally is not force-closed.
// Upstream d83951543 (#25258) + 25b35c972 (#25755).
func TestUBF074_ShutdownTimeout(t *testing.T) {
	t.Run("hung-connection-times-out", func(t *testing.T) {
		if shutdownTimeout > 10*time.Second {
			t.Fatalf("shutdownTimeout is %v, too long to be a bound", shutdownTimeout)
		}

		block := make(chan struct{})
		defer close(block)
		inHandler := make(chan struct{})

		srv := newHTTPServer(testlog.Logger(t, log.LvlDebug), rpc.DefaultHTTPTimeouts)
		srv.mux.HandleFunc("/hang", func(w http.ResponseWriter, r *http.Request) {
			close(inHandler)
			<-block
		})
		srv.handlerNames["/hang"] = "hang"
		// The mux is only consulted when the RPC handler is enabled.
		if err := srv.enableRPC(nil, httpConfig{}); err != nil {
			t.Fatal(err)
		}
		if err := srv.setListenAddr("127.0.0.1", 0); err != nil {
			t.Fatal(err)
		}
		if err := srv.start(); err != nil {
			t.Fatal(err)
		}

		// Start a request that never finishes, so graceful shutdown cannot complete.
		reqDone := make(chan struct{})
		go func() {
			defer close(reqDone)
			resp, err := http.Get("http://" + srv.listenAddr() + "/hang")
			if err == nil {
				resp.Body.Close()
			}
		}()
		select {
		case <-inHandler:
		case <-time.After(10 * time.Second):
			t.Fatal("request never reached the handler")
		}

		start := time.Now()
		srv.stop()
		elapsed := time.Since(start)
		if elapsed > shutdownTimeout+5*time.Second {
			t.Fatalf("stop took %v, expected it to be bounded by %v", elapsed, shutdownTimeout)
		}
		<-reqDone
	})

	t.Run("idle-server-is-not-force-closed", func(t *testing.T) {
		srv := createAndStartServer(t, &httpConfig{}, false, &wsConfig{})
		start := time.Now()
		srv.stop()
		if elapsed := time.Since(start); elapsed >= shutdownTimeout {
			t.Fatalf("stopping an idle server took %v, it should return immediately", elapsed)
		}
	})
}
