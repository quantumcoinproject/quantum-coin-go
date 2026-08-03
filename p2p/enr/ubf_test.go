// Copyright 2019 The go-ethereum Authors
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

package enr

import (
	"errors"
	"fmt"
	"testing"
)

// TestUBF104_WrappedErrorsRecognised checks that IsNotFound unwraps and that KeyError
// exposes the cause to errors.Is/errors.As. Upstream 138f0d749.
func TestUBF104_WrappedErrorsRecognised(t *testing.T) {
	var r Record
	err := r.Load(new(IP))
	if err == nil {
		t.Fatal("expected an error loading a missing key")
	}
	if !IsNotFound(err) {
		t.Fatal("IsNotFound does not recognise a plain *KeyError")
	}

	// Before the fix IsNotFound used a bare type assertion, so any wrapping hid the
	// *KeyError from it.
	wrapped := fmt.Errorf("loading record: %w", err)
	if !IsNotFound(wrapped) {
		t.Error("IsNotFound does not unwrap a wrapped *KeyError")
	}

	// Unwrap must expose the underlying cause.
	var ke *KeyError
	if !errors.As(wrapped, &ke) {
		t.Fatal("errors.As cannot find the *KeyError")
	}
	if !errors.Is(err, errNotFound) {
		t.Error("errors.Is(err, errNotFound) is false; KeyError.Unwrap is missing")
	}

	// Unrelated errors must not be reported as not-found.
	if IsNotFound(errors.New("boom")) {
		t.Error("IsNotFound reported true for an unrelated error")
	}
	if IsNotFound(nil) {
		t.Error("IsNotFound reported true for nil")
	}
}
