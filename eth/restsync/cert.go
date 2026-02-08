// Copyright 2020 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// Package restsync implements TLS certificate handling for the REST sync server.
// It uses hybrid PQC (Ed25519 + ML-DSA + SLH-DSA) certificates from crypto/tls.
package restsync

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	pqctls "github.com/quantumcoinproject/quantum-coin-go/crypto/tls"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/pqchelpereddsamldsaslhdsa"
	"github.com/quantumcoinproject/quantum-coin-go/log"
)

const (
	certFilename = "restsync-tls.crt"
	keyFilename  = "restsync-tls.key"
	certValidity = 365 * 24 * time.Hour
)

// ensureCertKey creates a self-signed PQC certificate and key in dataDir if they do not exist.
// Returns certFile, keyFile paths and any error.
// Cert is PEM-encoded; key file is raw secret key bytes (binary).
func ensureCertKey(dataDir string) (certFile, keyFile string, err error) {
	certFile = filepath.Join(dataDir, certFilename)
	keyFile = filepath.Join(dataDir, keyFilename)
	if _, errCert := os.Stat(certFile); errCert == nil {
		if _, errKey := os.Stat(keyFile); errKey == nil {
			return certFile, keyFile, nil
		}
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return "", "", fmt.Errorf("create data dir: %w", err)
	}
	publicKey, secretKey, err := pqchelpereddsamldsaslhdsa.GenerateKey()
	if err != nil {
		return "", "", fmt.Errorf("generate PQC key: %w", err)
	}
	_ = publicKey
	signer, err := pqctls.NewHybridSigner(secretKey)
	if err != nil {
		return "", "", fmt.Errorf("create signer: %w", err)
	}
	template, err := pqctls.DefaultCertTemplate(certValidity)
	if err != nil {
		return "", "", fmt.Errorf("cert template: %w", err)
	}
	template.CommonName = "restsync"
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
	log.Info("REST sync: created self-signed PQC TLS certificate", "dir", dataDir)
	return certFile, keyFile, nil
}

// loadCertKey loads the PQC certificate (PEM) and secret key (raw bytes) from files
// and returns the cert DER and a HybridSigner for use with TLS.
func loadCertKey(certFile, keyFile string) (certDER []byte, signer *pqctls.HybridSigner, err error) {
	pemBytes, err := os.ReadFile(certFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read cert: %w", err)
	}
	certDER, err = pqctls.PEMToCertDER(pemBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse cert PEM: %w", err)
	}
	secretKey, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read key: %w", err)
	}
	signer, err = pqctls.NewHybridSigner(secretKey)
	if err != nil {
		return nil, nil, fmt.Errorf("create signer from key: %w", err)
	}
	return certDER, signer, nil
}
