package hashingalgorithm

import (
	"bytes"
)
import "testing"

// Sum and Read are exclusive usage modes of a HashState: Sum finalizes a
// digest without consuming state, while Read squeezes an output stream.
// Calling Write or Sum after Read has started panics (x/crypto sha3
// contract). Sum may be followed by Read, since Sum does not advance the
// state.
func HashStateSumTest(t *testing.T, h HashState) {
	msg := []byte("abc")

	_, err := h.Write(msg)
	if err != nil {
		t.Fatal(err)
	}

	// Sum is idempotent and does not consume the state.
	d1 := h.Sum(nil)
	d2 := h.Sum(nil)
	if !bytes.Equal(d1, d2) {
		t.Fatal("Sum is not idempotent")
	}
	if len(d1) != h.Size() {
		t.Fatal("Sum output length mismatch")
	}

	// Sum appends to its argument.
	d3 := h.Sum(msg)
	if !bytes.Equal(d3[:len(msg)], msg) || !bytes.Equal(d3[len(msg):], d1) {
		t.Fatal("Sum does not append digest to its argument")
	}

	// The first Size() bytes of the Read stream equal the digest.
	d4 := make([]byte, h.Size())
	h.Read(d4)
	if !bytes.Equal(d1, d4) {
		t.Fatal("first Read block differs from Sum digest")
	}

	// Reset returns the state to absorbing mode.
	h.Reset()
	_, err = h.Write(msg)
	if err != nil {
		t.Fatal(err)
	}
	d5 := h.Sum(nil)
	if !bytes.Equal(d1, d5) {
		t.Fatal("digest differs after Reset")
	}

	// Reset also restores the Read stream from the beginning.
	d6 := make([]byte, h.Size())
	h.Read(d6)
	if !bytes.Equal(d1, d6) {
		t.Fatal("Read stream differs after Reset")
	}
}

func HashStateTest(t *testing.T, h1 HashState, h2 HashState) {
	msg := []byte("abc")

	d1 := make([]byte, h1.Size())
	d2 := make([]byte, h1.Size())
	d3 := make([]byte, h1.Size())
	d4 := make([]byte, h1.Size())

	_, err := h1.Write(msg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = h2.Write(msg)
	if err != nil {
		t.Fatal(err)
	}

	// Sum before Read is allowed and identical across instances.
	d5 := h1.Sum(msg)
	d6 := h2.Sum(msg)
	if !bytes.Equal(d5, d6) {
		t.Fatal("Sum differs between identical instances")
	}

	// The Read stream is deterministic across instances.
	h1.Read(d1)
	h2.Read(d2)
	if !bytes.Equal(d1, d2) {
		t.Fatal("first Read block differs between identical instances")
	}

	h1.Read(d3)
	h2.Read(d4)
	if !bytes.Equal(d3, d4) {
		t.Fatal("second Read block differs between identical instances")
	}
	if bytes.Equal(d1, d3) {
		t.Fatal("Read stream repeated a block")
	}
}
