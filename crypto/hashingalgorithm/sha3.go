package hashingalgorithm

import (
	"golang.org/x/crypto/sha3"
)

// Sha3HashState preserves the legacy x/crypto sha3 semantics this codebase
// was built against: Sum may be called after Read, in which case it returns
// the most recent Size() bytes squeezed by Read (without advancing the
// stream). Newer x/crypto versions panic on Sum-after-Read, so the last
// squeezed block is tracked here instead of relying on the sha3 state.
type Sha3HashState struct {
	sha3     HashState
	lastRead []byte
}

func NewSha3HashState() *Sha3HashState {
	return &Sha3HashState{
		sha3: sha3.NewLegacyKeccak256().(HashState),
	}
}

func (s *Sha3HashState) Write(p []byte) (n int, err error) {
	return s.sha3.Write(p)
}

func (s *Sha3HashState) Sum(b []byte) []byte {
	if s.lastRead == nil {
		return s.sha3.Sum(b)
	}
	return append(b, s.lastRead...)
}

func (s *Sha3HashState) Reset() {
	s.sha3.Reset()
	s.lastRead = nil
}

func (s *Sha3HashState) Size() int {
	return s.sha3.Size()
}

func (s *Sha3HashState) BlockSize() int {
	return s.sha3.BlockSize()
}

func (s *Sha3HashState) Read(b []byte) (int, error) {
	n, err := s.sha3.Read(b)
	if n > 0 {
		size := s.sha3.Size()
		combined := append(s.lastRead, b[:n]...)
		if len(combined) > size {
			combined = combined[len(combined)-size:]
		}
		s.lastRead = append([]byte(nil), combined...)
	}
	return n, err
}
