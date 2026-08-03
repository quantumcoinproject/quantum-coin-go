// Copyright 2014 The go-ethereum Authors
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

package p2p

import (
	"errors"
	"fmt"
	"io"

	"github.com/quantumcoinproject/quantum-coin-go/rlp"
)

const (
	errInvalidMsgCode = iota
	errInvalidMsg
)

var errorToString = map[int]string{
	errInvalidMsgCode: "invalid message code 1",
	errInvalidMsg:     "invalid message",
}

type peerError struct {
	code    int
	message string
}

func newPeerError(code int, format string, v ...interface{}) *peerError {
	desc, ok := errorToString[code]
	if !ok {
		panic("invalid error code")
	}
	err := &peerError{code, desc}
	if format != "" {
		err.message += ": " + fmt.Sprintf(format, v...)
	}
	return err
}

func (pe *peerError) Error() string {
	return pe.message
}

var errProtocolReturned = errors.New("protocol returned")

// DiscReason is the disconnect reason sent in a discMsg. Upstream 870b4505a
// (CVE-2022-29177): every other devp2p implementation stores this as a single
// byte, so decoding it as a wider integer let a peer supply out-of-range values.
type DiscReason uint8

const (
	DiscRequested DiscReason = iota
	DiscNetworkError
	DiscProtocolError
	DiscUselessPeer
	DiscTooManyPeers
	DiscAlreadyConnected
	DiscIncompatibleVersion
	DiscInvalidIdentity
	DiscQuitting
	DiscUnexpectedIdentity
	DiscSelf
	DiscReadTimeout
	DiscSubprotocolError = 0x10
)

var discReasonToString = [...]string{
	DiscRequested:           "disconnect requested",
	DiscNetworkError:        "network error",
	DiscProtocolError:       "breach of protocol",
	DiscUselessPeer:         "useless peer",
	DiscTooManyPeers:        "too many peers",
	DiscAlreadyConnected:    "already connected",
	DiscIncompatibleVersion: "incompatible p2p protocol version",
	DiscInvalidIdentity:     "invalid node identity",
	DiscQuitting:            "client quitting",
	DiscUnexpectedIdentity:  "unexpected identity",
	DiscSelf:                "connected to self",
	DiscReadTimeout:         "read timeout",
	DiscSubprotocolError:    "subprotocol error",
}

func (d DiscReason) String() string {
	if len(discReasonToString) <= int(d) {
		return fmt.Sprintf("unknown disconnect reason %d", d)
	}
	return discReasonToString[d]
}

func (d DiscReason) Error() string {
	return d.String()
}

// decodeDisconnectMessage decodes the payload of a discMsg. Upstream c1c250714:
// now that DiscReason is a single byte, the canonical `[]DiscReason{r}` encoding
// is a byte string rather than a list, and implementations differ over which of
// the two they send. Accept both, plus a bare byte, so a disconnect reason is
// never silently reported as DiscRequested. An unrecognised payload yields
// DiscInvalidIdentity's zero value with an error, and callers fall back.
func decodeDisconnectMessage(r io.Reader) (DiscReason, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}
	if len(raw) == 0 {
		return 0, io.EOF
	}
	// Canonical list form: a one-element list holding the reason.
	var asList struct{ R DiscReason }
	if err := rlp.DecodeBytes(raw, &asList); err == nil {
		return asList.R, nil
	}
	// Byte-string form, which is what []DiscReason encodes to now.
	var asBytes []byte
	if err := rlp.DecodeBytes(raw, &asBytes); err == nil && len(asBytes) == 1 {
		return DiscReason(asBytes[0]), nil
	}
	// Bare single byte, sent by some implementations.
	if len(raw) == 1 && raw[0] < 0x80 {
		return DiscReason(raw[0]), nil
	}
	return 0, errors.New("invalid disconnect message")
}

func discReasonForError(err error) DiscReason {
	if reason, ok := err.(DiscReason); ok {
		return reason
	}
	if errors.Is(err, errProtocolReturned) { // Upstream 138f0d749.
		return DiscQuitting
	}
	peerError, ok := err.(*peerError)
	if ok {
		switch peerError.code {
		case errInvalidMsgCode, errInvalidMsg:
			return DiscProtocolError
		default:
			return DiscSubprotocolError
		}
	}
	return DiscSubprotocolError
}
