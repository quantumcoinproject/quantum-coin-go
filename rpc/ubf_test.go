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

package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/quantumcoinproject/quantum-coin-go/log"
)

// TestUBF055_BatchLimits checks the server-side batch item-count and response-size
// limits. Upstream f3314bb6d (#26681).
func TestUBF055_BatchLimits(t *testing.T) {
	t.Run("item-limit", func(t *testing.T) {
		server := newTestServer()
		defer server.Stop()
		server.SetBatchLimits(2, 100000)
		client := DialInProc(server)
		defer client.Close()

		batch := []BatchElem{
			{Method: "test_echo", Args: []interface{}{"x", 1, nil}, Result: new(echoResult)},
			{Method: "test_echo", Args: []interface{}{"y", 2, nil}, Result: new(echoResult)},
			{Method: "test_echo", Args: []interface{}{"z", 3, nil}, Result: new(echoResult)},
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.BatchCallContext(ctx, batch); err != nil {
			t.Fatal("unexpected error:", err)
		}
		// The batch error is reported on the first call in the batch.
		var err0 Error
		if !errors.As(batch[0].Error, &err0) {
			t.Fatalf("batch elem 0 has wrong error type %T: %v", batch[0].Error, batch[0].Error)
		}
		if err0.ErrorCode() != -32600 || err0.Error() != errMsgBatchTooLarge {
			t.Fatalf("wrong error on batch elem 0: %v (code %d)", err0, err0.ErrorCode())
		}
		// The rest must be reported as absent rather than silently left blank.
		for i, elem := range batch[1:] {
			if elem.Error != ErrMissingBatchResponse {
				t.Fatalf("batch elem %d has unexpected error: %v", i+1, elem.Error)
			}
		}
	})

	t.Run("response-size-limit", func(t *testing.T) {
		server := newTestServer()
		defer server.Stop()
		// One echo result is well over 1 byte, so the second call must be rejected.
		server.SetBatchLimits(100, 1)
		client := DialInProc(server)
		defer client.Close()

		batch := []BatchElem{
			{Method: "test_echo", Args: []interface{}{"x", 1, nil}, Result: new(echoResult)},
			{Method: "test_echo", Args: []interface{}{"y", 2, nil}, Result: new(echoResult)},
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.BatchCallContext(ctx, batch); err != nil {
			t.Fatal("unexpected error:", err)
		}
		if batch[0].Error != nil {
			t.Fatalf("batch elem 0 should have succeeded, got %v", batch[0].Error)
		}
		var err1 Error
		if !errors.As(batch[1].Error, &err1) {
			t.Fatalf("batch elem 1 has wrong error type %T: %v", batch[1].Error, batch[1].Error)
		}
		if err1.ErrorCode() != errcodeResponseTooLarge || err1.Error() != errMsgResponseTooLarge {
			t.Fatalf("wrong error on batch elem 1: %v (code %d)", err1, err1.ErrorCode())
		}
	})

	t.Run("under-limit-still-works", func(t *testing.T) {
		server := newTestServer()
		defer server.Stop()
		client := DialInProc(server)
		defer client.Close()

		batch := []BatchElem{
			{Method: "test_echo", Args: []interface{}{"x", 1, nil}, Result: new(echoResult)},
			{Method: "test_echo", Args: []interface{}{"y", 2, nil}, Result: new(echoResult)},
		}
		if err := client.BatchCall(batch); err != nil {
			t.Fatal("unexpected error:", err)
		}
		for i, elem := range batch {
			if elem.Error != nil {
				t.Fatalf("batch elem %d failed: %v", i, elem.Error)
			}
		}
	})
}

// TestUBF056_ReadHeaderTimeoutSet checks that the default timeouts include a
// ReadHeaderTimeout, which bounds slowloris-style header dribbling.
// Upstream 9244f87dc (#25338).
func TestUBF056_ReadHeaderTimeoutSet(t *testing.T) {
	if DefaultHTTPTimeouts.ReadHeaderTimeout != 30*time.Second {
		t.Fatalf("DefaultHTTPTimeouts.ReadHeaderTimeout = %v, want 30s", DefaultHTTPTimeouts.ReadHeaderTimeout)
	}
}

// TestUBF058_WebsocketPongDeadline checks that a websocket codec installs a pong handler
// which clears the read deadline armed by the ping loop. Without it, a read deadline set
// before sending a ping is never cleared. Upstream 51ececb64 (#23556).
func TestUBF058_WebsocketPongDeadline(t *testing.T) {
	if wsPongTimeout != 30*time.Second {
		t.Fatalf("wsPongTimeout = %v, want 30s", wsPongTimeout)
	}

	var (
		upgrader = websocket.Upgrader{}
		codecCh  = make(chan *websocketCodec, 1)
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error("upgrade failed:", err)
			return
		}
		codecCh <- newWebsocketCodec(conn).(*websocketCodec)
	}))
	defer srv.Close()

	wsURL := "ws:" + strings.TrimPrefix(srv.URL, "http:")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal("dial failed:", err)
	}
	defer clientConn.Close()

	wc := <-codecCh
	defer wc.close()

	// The client must be reading for gorilla to answer the ping automatically.
	clientRead := make(chan error, 1)
	go func() {
		for {
			if _, _, err := clientConn.ReadMessage(); err != nil {
				clientRead <- err
				return
			}
		}
	}()

	// Do exactly what pingLoop does: send a ping and arm the pong deadline.
	wc.jsonCodec.encMu.Lock()
	wc.conn.SetWriteDeadline(time.Now().Add(wsPingWriteTimeout))
	if err := wc.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
		wc.jsonCodec.encMu.Unlock()
		t.Fatal("ping write failed:", err)
	}
	wc.conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	wc.jsonCodec.encMu.Unlock()

	// Reading processes the pong; the handler must clear the deadline.
	readErr := make(chan error, 1)
	go func() {
		var v interface{}
		readErr <- wc.conn.ReadJSON(&v)
	}()

	// Give the pong time to arrive and the deadline time to expire had it not been
	// cleared, then send a real message.
	time.Sleep(500 * time.Millisecond)
	if err := clientConn.WriteMessage(websocket.TextMessage, []byte(`{"jsonrpc":"2.0"}`)); err != nil {
		t.Fatal("client write failed:", err)
	}
	select {
	case err := <-readErr:
		if err != nil {
			t.Fatalf("read failed, pong did not clear the read deadline: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("read did not return")
	}
}

// TestUBF069_RejectsBadVersion checks that messages without "jsonrpc":"2.0" are rejected.
// Upstream 38e002f46 (#25570).
func TestUBF069_RejectsBadVersion(t *testing.T) {
	for _, vsnField := range []string{"", "2.1", "1.0", "go-ethereum"} {
		msg := &jsonrpcMessage{Version: vsnField, ID: json.RawMessage("1"), Method: "test_echo"}
		if msg.isCall() {
			t.Errorf("message with version %q accepted as a call", vsnField)
		}
		notif := &jsonrpcMessage{Version: vsnField, Method: "test_echo"}
		if notif.isNotification() {
			t.Errorf("message with version %q accepted as a notification", vsnField)
		}
		resp := &jsonrpcMessage{Version: vsnField, ID: json.RawMessage("1"), Result: json.RawMessage("1")}
		if resp.isResponse() {
			t.Errorf("message with version %q accepted as a response", vsnField)
		}
	}
	good := &jsonrpcMessage{Version: "2.0", ID: json.RawMessage("1"), Method: "test_echo"}
	if !good.isCall() {
		t.Fatal("valid call rejected")
	}

	// End-to-end: the server must answer with "invalid request".
	server := newTestServer()
	defer server.Stop()
	in, out := net.Pipe()
	go server.ServeCodec(NewCodec(out), 0)
	defer in.Close()

	in.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := in.Write([]byte(`{"jsonrpc":"2.1","id":1,"method":"test_echo","params":["x",3,null]}`)); err != nil {
		t.Fatal(err)
	}
	var resp jsonrpcMessage
	if err := json.NewDecoder(in).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error == nil || resp.Error.Code != -32600 {
		t.Fatalf("want invalid request (-32600), got %v", resp.Error)
	}
}

type panicService struct{}

func (panicService) Boom() (string, error) { panic("boom") }

// TestUBF071_InternalErrorCodes checks the finer-grained internal server error codes.
// Upstream 610cf02c4 (#25678).
func TestUBF071_InternalErrorCodes(t *testing.T) {
	server := NewServer()
	defer server.Stop()
	if err := server.RegisterName("panicsvc", panicService{}); err != nil {
		t.Fatal(err)
	}
	if err := server.RegisterName("nftest", new(notificationTestService)); err != nil {
		t.Fatal(err)
	}

	// A handler panic must be reported as -32603, not the generic -32000.
	client := DialInProc(server)
	defer client.Close()
	var out string
	err := client.Call(&out, "panicsvc_boom")
	rpcErr, ok := err.(Error)
	if !ok {
		t.Fatalf("want an rpc.Error, got %T: %v", err, err)
	}
	if rpcErr.ErrorCode() != errcodePanic {
		t.Errorf("panic error code = %d, want %d", rpcErr.ErrorCode(), errcodePanic)
	}

	// Subscribing over HTTP (where subscriptions are unavailable) must be -32001.
	httpSrv := httptest.NewServer(server)
	defer httpSrv.Close()
	resp, err := http.Post(httpSrv.URL, "application/json",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"nftest_subscribe","params":["someSubscription",0,0]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var msg jsonrpcMessage
	if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
		t.Fatal(err)
	}
	if msg.Error == nil || msg.Error.Code != errcodeNotificationsUnsupported {
		t.Errorf("subscribe-over-HTTP error = %v, want code %d", msg.Error, errcodeNotificationsUnsupported)
	}
}

// TestUBF072_BatchMatchingIndexed checks that batch responses are matched to requests by
// ID through an index rather than a linear scan, including when the server reorders them.
// Upstream 53b94f135 (#23856).
func TestUBF072_BatchMatchingIndexed(t *testing.T) {
	const n = 50
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqs []jsonrpcMessage
		if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
			t.Error(err)
			return
		}
		// Answer in reverse order to prove matching is by ID, not by position.
		resps := make([]jsonrpcMessage, 0, len(reqs))
		for i := len(reqs) - 1; i >= 0; i-- {
			resps = append(resps, jsonrpcMessage{
				Version: vsn,
				ID:      reqs[i].ID,
				Result:  json.RawMessage(`"` + reqs[i].Method + `"`),
			})
		}
		json.NewEncoder(w).Encode(resps)
	}))
	defer srv.Close()

	client, err := Dial(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	batch := make([]BatchElem, n)
	for i := range batch {
		batch[i] = BatchElem{Method: "m" + strconv.Itoa(i), Result: new(string)}
	}
	if err := client.BatchCall(batch); err != nil {
		t.Fatal(err)
	}
	for i, elem := range batch {
		if elem.Error != nil {
			t.Fatalf("elem %d: %v", i, elem.Error)
		}
		if got := *elem.Result.(*string); got != elem.Method {
			t.Fatalf("elem %d matched to the wrong response: got %q, want %q", i, got, elem.Method)
		}
	}
}

// TestUBF073_BatchLengthValidated checks that a batch response of the wrong length is
// reported instead of leaving the caller waiting. Upstream 05037eaff (#26064).
func TestUBF073_BatchLengthValidated(t *testing.T) {
	body, err := json.Marshal([]jsonrpcMessage{
		{Version: vsn, ID: json.RawMessage("1"), Result: json.RawMessage(`"0x1"`)},
		{Version: vsn, ID: json.RawMessage("2"), Result: json.RawMessage(`"0x2"`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	defer srv.Close()

	for _, size := range []int{1, 3} {
		client, err := Dial(srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		batch := make([]BatchElem, size)
		for i := range batch {
			batch[i] = BatchElem{Method: "foo", Result: new(string)}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = client.BatchCallContext(ctx, batch)
		cancel()
		client.Close()
		if !errors.Is(err, ErrBadResult) {
			t.Fatalf("batch of %d: want ErrBadResult, got %v", size, err)
		}
	}
}

// TestUBF075_LogDurationKey checks that the served-call timing is logged under "duration"
// rather than "t", which collides with the logger's own timestamp key.
// Upstream 0ba0b81e5 (#24112).
func TestUBF075_LogDurationKey(t *testing.T) {
	var (
		mu   sync.Mutex
		keys []string
	)
	old := log.Root().GetHandler()
	log.Root().SetHandler(log.FuncHandler(func(r *log.Record) error {
		mu.Lock()
		defer mu.Unlock()
		for i := 0; i+1 < len(r.Ctx); i += 2 {
			if k, ok := r.Ctx[i].(string); ok {
				keys = append(keys, k)
			}
		}
		return nil
	}))
	defer log.Root().SetHandler(old)

	server := newTestServer()
	defer server.Stop()
	client := DialInProc(server)
	defer client.Close()

	var res echoResult
	if err := client.Call(&res, "test_echo", "x", 3, nil); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	var sawDuration bool
	for _, k := range keys {
		if k == "t" {
			t.Errorf("log key %q is still used; it collides with the logger timestamp", k)
		}
		if k == "duration" {
			sawDuration = true
		}
	}
	if !sawDuration {
		t.Error(`no log record used the "duration" key`)
	}
}

// recordingTransport captures the outgoing request and returns a canned response.
type recordingTransport struct {
	req    *http.Request
	status int
	body   *closeTrackingBody
}

func (rt *recordingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	rt.req = r
	status := rt.status
	if status == 0 {
		status = http.StatusOK
	}
	rt.body = &closeTrackingBody{Reader: strings.NewReader(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`)}
	return &http.Response{
		Status:     http.StatusText(status),
		StatusCode: status,
		Body:       rt.body,
		Header:     make(http.Header),
	}, nil
}

type closeTrackingBody struct {
	io.Reader
	closed bool
}

func (b *closeTrackingBody) Close() error {
	b.closed = true
	return nil
}

// TestUBF076_RequestGetBodySet checks that outgoing requests carry GetBody, which HTTP/2
// needs in order to replay a request after a GOAWAY. Upstream abd49a6c4 (#24292).
func TestUBF076_RequestGetBodySet(t *testing.T) {
	rt := new(recordingTransport)
	client, err := DialHTTPWithClient("http://example.invalid", &http.Client{Transport: rt})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	var out string
	if err := client.Call(&out, "test_echo"); err != nil {
		t.Fatal(err)
	}
	if rt.req.GetBody == nil {
		t.Fatal("Request.GetBody is nil")
	}
	body, err := rt.req.GetBody()
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(replayed) == 0 || int64(len(replayed)) != rt.req.ContentLength {
		t.Fatalf("GetBody returned %d bytes, want %d", len(replayed), rt.req.ContentLength)
	}
}

// TestUBF077_NullResult checks that a JSON 'null' result lands in the caller's value
// instead of being unmarshaled into a pointer to the interface. Upstream 1db978ca6 (#26723).
func TestUBF077_NullResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonrpcMessage
		json.NewDecoder(r.Body).Decode(&req)
		json.NewEncoder(w).Encode(jsonrpcMessage{Version: vsn, ID: req.ID, Result: json.RawMessage("null")})
	}))
	defer srv.Close()

	client, err := Dial(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	var result json.RawMessage
	if err := client.Call(&result, "test_null"); err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !reflect.DeepEqual(result, json.RawMessage("null")) {
		t.Errorf("expected null, got %s", result)
	}

	// A nil result argument must be accepted and simply discard the response.
	if err := client.Call(nil, "test_null"); err != nil {
		t.Fatal(err)
	}
}

// unsubscribeBlocker blocks forever on unsubscribe requests.
type unsubscribeBlocker struct {
	ServerCodec
	quit chan struct{}
}

func (b *unsubscribeBlocker) readBatch() ([]*jsonrpcMessage, bool, error) {
	msgs, batch, err := b.ServerCodec.readBatch()
	for _, msg := range msgs {
		if msg.isUnsubscribe() {
			<-b.quit
		}
	}
	return msgs, batch, err
}

// TestUBF078_UnsubscribeTimeout checks that Unsubscribe eventually returns even when the
// server never answers the *_unsubscribe call. Upstream 15fb0dcc6 (#30318).
func TestUBF078_UnsubscribeTimeout(t *testing.T) {
	srv := NewServer()
	if err := srv.RegisterName("nftest", new(notificationTestService)); err != nil {
		t.Fatal(err)
	}

	p1, p2 := net.Pipe()
	blocker := &unsubscribeBlocker{ServerCodec: NewCodec(p1), quit: make(chan struct{})}
	defer close(blocker.quit)

	go srv.ServeCodec(blocker, 0)
	defer srv.Stop()

	client, _ := newClient(context.Background(), func(context.Context) (ServerCodec, error) {
		return NewCodec(p2), nil
	})
	defer client.Close()

	sub, err := client.Subscribe(context.Background(), "nftest", make(chan int), "someSubscription", 1, 1)
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	done := make(chan struct{})
	go func() {
		sub.Unsubscribe()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(unsubscribeTimeout + 5*time.Second):
		t.Fatalf("Unsubscribe did not return within %s", unsubscribeTimeout)
	}
}

// TestUBF079_ErrorResponseBodyClosed checks that the response body is closed when a non-2xx
// status turns into an HTTPError, so the connection is not leaked.
// Upstream 99bbbc027 (#29223).
func TestUBF079_ErrorResponseBodyClosed(t *testing.T) {
	rt := &recordingTransport{status: http.StatusInternalServerError}
	client, err := DialHTTPWithClient("http://example.invalid", &http.Client{Transport: rt})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	var out string
	err = client.Call(&out, "test_echo")
	if _, ok := err.(HTTPError); !ok {
		t.Fatalf("want HTTPError, got %T: %v", err, err)
	}
	if !rt.body.closed {
		t.Fatal("response body was not closed on the error path")
	}
}
