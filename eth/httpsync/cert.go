// Copyright 2020 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// Package httpsync implements TLS certificate handling for the HTTP sync server.
// It uses hybrid PQC (Ed25519 + ML-DSA + SLH-DSA) certificates from crypto/tls.
// When the node key is the compact PQC type (same as TLS), the cert is created from it and no separate key file is written.
package httpsync

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/signaturealgorithm"
	pqctls "github.com/quantumcoinproject/quantum-coin-go/crypto/tls"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/pqchelpereddsamldsaslhdsa"
	"github.com/quantumcoinproject/quantum-coin-go/log"
)

const (
	certFilename       = "httpsync-tls.crt"
	keyFilename       = "httpsync-tls.key"
	certValidity      = 365 * 24 * time.Hour
	certRenewBefore   = 30 * 24 * time.Hour // regenerate cert if expiry is within 30 days
	certCheckInterval = 24 * time.Hour      // how often to check cert expiry (renewal goroutine)
)

// compactPrivateKeyLength is the length of the raw private key for the hybrid compact scheme (same as TLS).
func compactPrivateKeyLength() int {
	return cryptobase.SigAlgHybridMlDsaEddsaSlhDsaCompact.PrivateKeyLength()
}

// nodeKeyUsableForTLS returns true if the node key is the compact PQC type (same raw key format as TLS).
func nodeKeyUsableForTLS(nodeKey *signaturealgorithm.PrivateKey) bool {
	return nodeKey != nil && len(nodeKey.PriData) == compactPrivateKeyLength()
}

// certExpiresSoon returns true if the certificate at certFile exists and expires within certRenewBefore.
func certExpiresSoon(certFile string) bool {
	pemBytes, err := os.ReadFile(certFile)
	if err != nil {
		return true // treat missing/unreadable as "expires soon" so we regenerate
	}
	certDER, err := pqctls.PEMToCertDER(pemBytes)
	if err != nil {
		return true
	}
	notAfter, err := pqctls.CertNotAfter(certDER)
	if err != nil {
		return true
	}
	return time.Until(notAfter) < certRenewBefore
}

// defaultCertCommonName is used when peerID (ENR node ID) is not available.
const defaultCertCommonName = "httpsync"

// EnsureCertKeyForTest creates a PQC cert in dataDir (for tests). Returns cert and key file paths.
func EnsureCertKeyForTest(dataDir string) (certFile, keyFile string, err error) {
	return ensureCertKey(dataDir, nil, "")
}

// ensureCertKey creates a self-signed PQC certificate in dataDir if it does not exist or expires within 30 days.
// commonName is the cert Subject CN (e.g. node peer ID from ENR); if empty, defaultCertCommonName is used.
// If nodeKey is the compact PQC type (same as TLS), the cert is signed with it and no key file is written (keyFile returned as "").
// Otherwise a new PQC key is generated and both cert and key files are written.
// Returns certFile, keyFile paths (keyFile may be "" when using nodeKey) and any error.
func ensureCertKey(dataDir string, nodeKey *signaturealgorithm.PrivateKey, commonName string) (certFile, keyFile string, err error) {
	if commonName == "" {
		commonName = defaultCertCommonName
	}
	certFile = filepath.Join(dataDir, certFilename)
	keyFile = filepath.Join(dataDir, keyFilename)
	if _, errCert := os.Stat(certFile); errCert == nil && !certExpiresSoon(certFile) {
		if nodeKeyUsableForTLS(nodeKey) {
			return certFile, "", nil
		}
		if _, errKey := os.Stat(keyFile); errKey == nil {
			return certFile, keyFile, nil
		}
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return "", "", fmt.Errorf("create data dir: %w", err)
	}
	var secretKey []byte
	useNodeKey := nodeKeyUsableForTLS(nodeKey)
	if useNodeKey {
		secretKey = make([]byte, len(nodeKey.PriData))
		copy(secretKey, nodeKey.PriData)
	} else {
		publicKey, sk, err := pqchelpereddsamldsaslhdsa.GenerateKey()
		if err != nil {
			return "", "", fmt.Errorf("generate PQC key: %w", err)
		}
		_ = publicKey
		secretKey = sk
	}
	signer, err := pqctls.NewHybridSigner(secretKey)
	if err != nil {
		return "", "", fmt.Errorf("create signer: %w", err)
	}
	template, err := pqctls.DefaultCertTemplate(certValidity)
	if err != nil {
		return "", "", fmt.Errorf("cert template: %w", err)
	}
	template.CommonName = commonName
	template.Organization = "quantum-coin"
	certDER, err := pqctls.CreateCertificate(template, signer)
	if err != nil {
		return "", "", fmt.Errorf("create certificate: %w", err)
	}
	certOut, err := os.Create(certFile)
	if err != nil {
		return "", "", fmt.Errorf("write cert: %w", err)
	}
	if _, err := certOut.Write(pqctls.CertificatePEM(certDER)); err != nil {
		certOut.Close()
		os.Remove(certFile)
		return "", "", err
	}
	certOut.Close()
	if useNodeKey {
		log.Info("HTTP sync: created self-signed PQC TLS certificate from node key", "dir", dataDir)
		return certFile, "", nil
	}
	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		os.Remove(certFile)
		return "", "", fmt.Errorf("write key: %w", err)
	}
	if _, err := keyOut.Write(secretKey); err != nil {
		keyOut.Close()
		os.Remove(certFile)
		os.Remove(keyFile)
		return "", "", err
	}
	keyOut.Close()
	log.Info("HTTP sync: created self-signed PQC TLS certificate", "dir", dataDir)
	return certFile, keyFile, nil
}

// loadCertKey loads the PQC certificate (PEM) and builds a signer from keyFile or from nodeKey when keyFile is "".
// When keyFile is "" and nodeKey is the compact PQC type, the signer is created from nodeKey.PriData.
func loadCertKey(certFile, keyFile string, nodeKey *signaturealgorithm.PrivateKey) (certDER []byte, signer *pqctls.HybridSigner, err error) {
	pemBytes, err := os.ReadFile(certFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read cert: %w", err)
	}
	certDER, err = pqctls.PEMToCertDER(pemBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse cert PEM: %w", err)
	}
	var secretKey []byte
	if keyFile == "" && nodeKeyUsableForTLS(nodeKey) {
		secretKey = make([]byte, len(nodeKey.PriData))
		copy(secretKey, nodeKey.PriData)
	} else {
		secretKey, err = os.ReadFile(keyFile)
		if err != nil {
			return nil, nil, fmt.Errorf("read key: %w", err)
		}
	}
	signer, err = pqctls.NewHybridSigner(secretKey)
	if err != nil {
		return nil, nil, fmt.Errorf("create signer from key: %w", err)
	}
	return certDER, signer, nil
}

// ClientTLSConfig builds a TLS config for the HTTP sync client: client cert (from dataDir, same as server)
// and server cert verification via cryptobase.DynamicVerifier (VerifyPeerCertificatesPQC).
// Ensures the cert exists (ensureCertKey with default CN) when used without a server.
func ClientTLSConfig(dataDir string, nodeKey *signaturealgorithm.PrivateKey) (*tls.Config, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("dataDir required for client cert")
	}
	if _, _, err := ensureCertKey(dataDir, nodeKey, ""); err != nil {
		return nil, err
	}
	certFile := filepath.Join(dataDir, certFilename)
	keyFile := filepath.Join(dataDir, keyFilename)
	certDER, signer, err := loadCertKey(certFile, keyFile, nodeKey)
	if err != nil {
		return nil, err
	}
	tlsCert := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  signer,
	}
	return &tls.Config{
		MinVersion:         tls.VersionTLS13,
		Certificates:       []tls.Certificate{tlsCert},
		InsecureSkipVerify: true, // we verify server cert in VerifyPeerCertificate with PQC
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			return pqctls.VerifyPeerCertificatesPQC(rawCerts)
		},
	}, nil
}

// ClientTLSConfigFromFiles builds a client TLS config from cert and key PEM files.
// Verifies server cert with VerifyPeerCertificatesPQC.
func ClientTLSConfigFromFiles(certFile, keyFile string) (*tls.Config, error) {
	certDER, signer, err := loadCertKey(certFile, keyFile, nil)
	if err != nil {
		return nil, err
	}
	tlsCert := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  signer,
	}
	return &tls.Config{
		MinVersion:         tls.VersionTLS13,
		Certificates:       []tls.Certificate{tlsCert},
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			return pqctls.VerifyPeerCertificatesPQC(rawCerts)
		},
	}, nil
}

// clientCertCommonName is the CN used for ephemeral in-memory client certs.
const clientCertCommonName = "httpsync-cli"

// ClientTLSConfigInMemory generates a new PQC keypair and self-signed cert in memory and returns a client TLS config.
// Use for one-off clients (e.g. CLI) that do not need to persist a cert. Verifies server cert with VerifyPeerCertificatesPQC.
func ClientTLSConfigInMemory() (*tls.Config, error) {
	publicKey, secretKey, err := pqchelpereddsamldsaslhdsa.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("generate PQC key: %w", err)
	}
	_ = publicKey
	signer, err := pqctls.NewHybridSigner(secretKey)
	if err != nil {
		return nil, fmt.Errorf("create signer: %w", err)
	}
	template, err := pqctls.DefaultCertTemplate(certValidity)
	if err != nil {
		return nil, fmt.Errorf("cert template: %w", err)
	}
	template.CommonName = clientCertCommonName
	template.Organization = "quantum-coin"
	certDER, err := pqctls.CreateCertificate(template, signer)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}
	tlsCert := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  signer,
	}
	return &tls.Config{
		MinVersion:         tls.VersionTLS13,
		Certificates:       []tls.Certificate{tlsCert},
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			return pqctls.VerifyPeerCertificatesPQC(rawCerts)
		},
	}, nil
}
