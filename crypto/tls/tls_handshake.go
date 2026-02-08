package tls

import (
	"bytes"
	"crypto/sha256"
	"hash"
	"io"
)

// TLS 1.3 CertificateVerify signed message format (RFC 8446, Section 4.4.3).
const (
	ServerSignatureContextTLS13 = "TLS 1.3, server CertificateVerify\x00"
	ClientSignatureContextTLS13 = "TLS 1.3, client CertificateVerify\x00"
)

var signaturePaddingTLS13 = []byte{
	0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
	0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
	0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
	0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
	0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
	0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
	0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
	0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
}

// SignedMessageTLS13 builds the (unhashed) message that the server signs for
// TLS 1.3 CertificateVerify (RFC 8446, Section 4.4.3). contextString is
// ServerSignatureContextTLS13 or ClientSignatureContextTLS13. transcriptHash is
// the SHA-256 of the handshake transcript up to and including the Certificate message.
//
// The hybrid PQC signer expects a 32-byte digest. When using HybridSigner for
// CertificateVerify, hash this message with SHA-256 and pass the 32-byte digest
// to Sign; the client must verify the same digest.
func SignedMessageTLS13(contextString string, transcriptHash []byte) []byte {
	const maxSize = 64 + 64 + 32
	b := bytes.NewBuffer(make([]byte, 0, maxSize))
	b.Write(signaturePaddingTLS13)
	io.WriteString(b, contextString)
	b.Write(transcriptHash)
	return b.Bytes()
}

// TranscriptHashSHA256 returns the SHA-256 hash of the concatenated handshake
// messages. For testing, you can pass the raw handshake bytes.
func TranscriptHashSHA256(messages ...[]byte) []byte {
	h := sha256.New()
	for _, m := range messages {
		h.Write(m)
	}
	return h.Sum(nil)
}

// NewTranscriptHash returns a hash.Hash (SHA-256) for building a handshake transcript.
func NewTranscriptHash() hash.Hash {
	return sha256.New()
}
