// Copyright 2016 The go-ethereum Authors
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

package event

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

var errInts = errors.New("error in subscribeInts")

func subscribeInts(max, fail int, c chan<- int) Subscription {
	return NewSubscription(func(quit <-chan struct{}) error {
		for i := 0; i < max; i++ {
			if i >= fail {
				return errInts
			}
			select {
			case c <- i:
			case <-quit:
				return nil
			}
		}
		return nil
	})
}

func TestNewSubscriptionError(t *testing.T) {
	t.Parallel()

	channel := make(chan int)
	sub := subscribeInts(10, 2, channel)
loop:
	for want := 0; want < 10; want++ {
		select {
		case got := <-channel:
			if got != want {
				t.Fatalf("wrong int %d, want %d", got, want)
			}
		case err := <-sub.Err():
			if err != errInts {
				t.Fatalf("wrong error: got %q, want %q", err, errInts)
			}
			if want != 2 {
				t.Fatalf("got errInts at int %d, should be received at 2", want)
			}
			break loop
		}
	}
	sub.Unsubscribe()

	err, ok := <-sub.Err()
	if err != nil {
		t.Fatal("got non-nil error after Unsubscribe")
	}
	if ok {
		t.Fatal("channel still open after Unsubscribe")
	}
}

func TestResubscribe(t *testing.T) {
	t.Parallel()

	var i int
	nfails := 6
	sub := Resubscribe(100*time.Millisecond, func(ctx context.Context) (Subscription, error) {
		// fmt.Printf("call #%d @ %v\n", i, time.Now())
		i++
		if i == 2 {
			// Delay the second failure a bit to reset the resubscribe interval.
			time.Sleep(200 * time.Millisecond)
		}
		if i < nfails {
			return nil, errors.New("oops")
		}
		sub := NewSubscription(func(unsubscribed <-chan struct{}) error { return nil })
		return sub, nil
	})

	<-sub.Err()
	if i != nfails {
		t.Fatalf("resubscribe function called %d times, want %d times", i, nfails)
	}
}

func TestResubscribeAbort(t *testing.T) {
	t.Parallel()

	done := make(chan error, 1)
	sub := Resubscribe(0, func(ctx context.Context) (Subscription, error) {
		select {
		case <-ctx.Done():
			done <- nil
		case <-time.After(2 * time.Second):
			done <- errors.New("context given to resubscribe function not canceled within 2s")
		}
		return nil, nil
	})

	sub.Unsubscribe()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestResubscribeWithErrorHandler(t *testing.T) {
	t.Parallel()

	var i int
	nfails := 6
	subErrs := make([]string, 0)
	sub := ResubscribeErr(100*time.Millisecond, func(ctx context.Context, lastErr error) (Subscription, error) {
		i++
		var lastErrVal string
		if lastErr != nil {
			lastErrVal = lastErr.Error()
		}
		subErrs = append(subErrs, lastErrVal)
		sub := NewSubscription(func(unsubscribed <-chan struct{}) error {
			if i < nfails {
				return fmt.Errorf("err-%v", i)
			} else {
				return nil
			}
		})
		return sub, nil
	})

	<-sub.Err()
	if i != nfails {
		t.Fatalf("resubscribe function called %d times, want %d times", i, nfails)
	}

	expectedSubErrs := []string{"", "err-1", "err-2", "err-3", "err-4", "err-5"}
	if !reflect.DeepEqual(subErrs, expectedSubErrs) {
		t.Fatalf("unexpected subscription errors %v, want %v", subErrs, expectedSubErrs)
	}
}

// TestUBF125_ResubscribeUnsubscribeAfterCompletion checks that Unsubscribe returns even
// when the resubscribe loop has already exited because the inner subscription ended
// successfully. Upstream ffc6a0f36: the unsub channel was unbuffered, so the send in
// Unsubscribe had no reader left and blocked forever.
func TestUBF125_ResubscribeUnsubscribeAfterCompletion(t *testing.T) {
	started := make(chan struct{})
	sub := ResubscribeErr(100*time.Millisecond, func(ctx context.Context, lastErr error) (Subscription, error) {
		return NewSubscription(func(unsubscribed <-chan struct{}) error {
			close(started)
			return nil // ends successfully, which terminates the resubscribe loop
		}), nil
	})

	<-started
	// Wait for the loop goroutine to exit. Err is closed by loop's defer.
	select {
	case <-sub.Err():
	case <-time.After(5 * time.Second):
		t.Fatal("resubscribe loop did not exit")
	}

	done := make(chan struct{})
	go func() {
		sub.Unsubscribe()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Unsubscribe deadlocked after the subscription ended")
	}
}
