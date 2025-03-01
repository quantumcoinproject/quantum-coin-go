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

package enode

import (
	"encoding/binary"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumCoinProject/qc/p2p/enr"
)

func TestReadNodes(t *testing.T) {
	nodes := ReadNodes(new(genIter), 10)
	checkNodes(t, nodes, 10)
}

// This test checks that ReadNodes terminates when reading N nodes from an iterator
// which returns less than N nodes in an endless cycle.
func TestReadNodesCycle(t *testing.T) {
	iter := &callCountIter{
		Iterator: CycleNodes([]*Node{
			testNode(0, 0),
			testNode(1, 0),
			testNode(2, 0),
		}),
	}
	nodes := ReadNodes(iter, 10)
	checkNodes(t, nodes, 3)
	if iter.count != 10 {
		t.Fatalf("%d calls to Next, want %d", iter.count, 100)
	}
}

func TestFilterNodes(t *testing.T) {
	nodes := make([]*Node, 100)
	for i := range nodes {
		nodes[i] = testNode(uint64(i), uint64(i))
	}

	it := Filter(IterNodes(nodes), func(n *Node) bool {
		return n.Seq() >= 50
	})
	for i := 50; i < len(nodes); i++ {
		if !it.Next() {
			t.Fatal("Next returned false")
		}
		if it.Node() != nodes[i] {
			t.Fatalf("iterator returned wrong node %v\nwant %v", it.Node(), nodes[i])
		}
	}
	if it.Next() {
		t.Fatal("Next returned true after underlying iterator has ended")
	}
}

func checkNodes(t *testing.T, nodes []*Node, wantLen int) {
	if len(nodes) != wantLen {
		t.Errorf("slice has %d nodes, want %d", len(nodes), wantLen)
		return
	}
	seen := make(map[ID]bool)
	for i, e := range nodes {
		if e == nil {
			t.Errorf("nil node at index %d", i)
			return
		}
		if seen[e.ID()] {
			t.Errorf("slice has duplicate node %v", e.ID())
			return
		}
		seen[e.ID()] = true
	}
}

type blockingIter chan struct{}

func (it blockingIter) Next() bool {
	<-it
	return false
}

func (it blockingIter) Node() *Node {
	return nil
}

func (it blockingIter) Close() {
	close(it)
}

func TestFairMixClose(t *testing.T) {
	for i := 0; i < 20 && !t.Failed(); i++ {
		testMixerClose(t)
	}
}

func testMixerClose(t *testing.T) {
	mix := NewFairMix(-1)
	mix.AddSource(CycleNodes(nil), "test")
	mix.AddSource(CycleNodes(nil), "test")

	done := make(chan struct{})
	go func() {
		defer close(done)
		if mix.Next() {
			t.Error("Next returned true")
		}
	}()
	// This call is supposed to make it more likely that NextNode is
	// actually executing by the time we call Close.
	runtime.Gosched()

	mix.Close()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Next didn't unblock on Close")
	}

	mix.Close() // shouldn't crash
}

func idPrefixDistribution(nodes []*Node) map[uint32]int {
	d := make(map[uint32]int)
	for _, node := range nodes {
		id := node.ID()
		d[binary.BigEndian.Uint32(id[:4])]++
	}
	return d
}

func approxEqual(x, y, ε int) bool {
	if y > x {
		x, y = y, x
	}
	return x-y > ε
}

// genIter creates fake nodes with numbered IDs based on 'index' and 'gen'
type genIter struct {
	node       *Node
	index, gen uint32
}

func (s *genIter) Next() bool {
	index := atomic.LoadUint32(&s.index)
	if index == ^uint32(0) {
		s.node = nil
		return false
	}
	s.node = testNode(uint64(index)<<32|uint64(s.gen), 0)
	s.gen++
	return true
}

func (s *genIter) Node() *Node {
	return s.node
}

func (s *genIter) Close() {
	s.index = ^uint32(0)
}

func testNode(id, seq uint64) *Node {
	var nodeID ID
	binary.BigEndian.PutUint64(nodeID[:], id)
	r := new(enr.Record)
	r.SetSeq(seq)
	return SignNull(r, nodeID)
}

// callCountIter counts calls to NextNode.
type callCountIter struct {
	Iterator
	count int
}

func (it *callCountIter) Next() bool {
	it.count++
	return it.Iterator.Next()
}
