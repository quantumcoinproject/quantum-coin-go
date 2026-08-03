// Copyright 2021 The go-ethereum Authors
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

package bloombits

import (
	"context"
	"errors"
	"testing"
	"time"
)

// ubfRetrievalError is a second, distinct error implementation. Storing it into
// the same atomic.Value as a plain errors.errorString is what used to panic.
type ubfRetrievalError struct{ msg string }

func (e ubfRetrievalError) Error() string { return e.msg }

// TestUBF012_MatcherDifferentErrorTypes covers upstream 8e0771c21: two retrieval
// failures with different dynamic error types inside the same MatcherSession
// used to panic with "store of inconsistently typed value into Value", because
// the failures were kept in an atomic.Value.
func TestUBF012_MatcherDifferentErrorTypes(t *testing.T) {
	// Drive Multiplex against a stubbed matcher so that the two failures can be
	// delivered simultaneously, which is what triggers the second, differently
	// typed store.
	matcher := NewMatcher(testSectionSize, nil)
	matcher.addScheduler(0)

	session := &MatcherSession{
		matcher: matcher,
		quit:    make(chan struct{}),
		ctx:     context.Background(),
	}
	stop := make(chan struct{})
	defer close(stop)
	go stubDistributor(matcher, stop)

	requests := make(chan chan *Retrieval)
	for i := 0; i < 2; i++ {
		go session.Multiplex(0, time.Microsecond, requests)
	}
	// Collect both in-flight retrievals before answering either of them.
	var (
		pending []chan *Retrieval
		tasks   []*Retrieval
		errFoo  = errors.New("first failure")
		errBar  = ubfRetrievalError{msg: "second failure"}
	)
	for len(pending) < 2 {
		select {
		case request := <-requests:
			task := <-request
			pending = append(pending, request)
			tasks = append(tasks, task)
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for retrieval requests")
		}
	}
	tasks[0].Error = errFoo
	tasks[1].Error = errBar
	pending[0] <- tasks[0]
	pending[1] <- tasks[1]

	// Both multiplexers now record their failure; the pre-fix code panicked on
	// the second store because its dynamic type differed from the first.
	deadline := time.Now().Add(10 * time.Second)
	for {
		if got := session.Error(); got != nil {
			if !errors.Is(got, errFoo) && got != error(errBar) {
				t.Fatalf("unexpected session error: %v", got)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("session error never recorded")
		}
		time.Sleep(time.Millisecond)
	}
}

// stubDistributor stands in for Matcher.distributor, handing out bit 0 to every
// requesting multiplexer and swallowing all deliveries.
func stubDistributor(m *Matcher, stop chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case fetcher := <-m.retrievers:
			fetcher <- 0
		case fetcher := <-m.counters:
			<-fetcher
			fetcher <- 1
		case fetcher := <-m.retrievals:
			task := <-fetcher
			task.Sections = []uint64{0}
			fetcher <- task
		case <-m.deliveries:
		}
	}
}
