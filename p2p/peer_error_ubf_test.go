package p2p

import (
	"bytes"
	"testing"

	"github.com/quantumcoinproject/quantum-coin-go/rlp"
)

// TestUBF101_DiscReasonRoundTrip pins the encode/decode pair together. Upstream
// 870b4505a made DiscReason a single byte, which silently turned []DiscReason{r}
// into an RLP byte string; c1c250714 restored the list encoding and made the
// decoder accept both. Without both halves a disconnect reason reads back as 0.
func TestUBF101_DiscReasonRoundTrip(t *testing.T) {
	for _, want := range []DiscReason{DiscRequested, DiscTooManyPeers, DiscQuitting, DiscSubprotocolError} {
		// The exact bytes the transport writes.
		var buf bytes.Buffer
		if err := rlp.Encode(&buf, []any{want}); err != nil {
			t.Fatalf("encode %v: %v", want, err)
		}
		got, err := decodeDisconnectMessage(bytes.NewReader(buf.Bytes()))
		if err != nil {
			t.Fatalf("decode %v: %v", want, err)
		}
		if got != want {
			t.Errorf("list form: got %v (%d), want %v (%d)", got, got, want, want)
		}

		// Byte-string form, which is what a peer running the pre-c1c250714
		// encoding sends. Must still decode to the same reason.
		buf.Reset()
		if err := rlp.Encode(&buf, []DiscReason{want}); err != nil {
			t.Fatalf("encode bytes %v: %v", want, err)
		}
		got, err = decodeDisconnectMessage(bytes.NewReader(buf.Bytes()))
		if err != nil {
			t.Fatalf("decode bytes %v: %v", want, err)
		}
		if got != want {
			t.Errorf("byte-string form: got %v (%d), want %v (%d)", got, got, want, want)
		}
	}
}
