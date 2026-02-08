package tls

import (
	"bytes"
	"math/big"
	"testing"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/crypto/pqchelpereddsamldsaslhdsa"
)

func TestCreateAndVerifyCertificatePQC(t *testing.T) {
	_, secret, err := pqchelpereddsamldsaslhdsa.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	signer, err := NewHybridSigner(secret)
	if err != nil {
		t.Fatal(err)
	}
	template, err := DefaultCertTemplate(24 * time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	template.CommonName = "test-pqc"
	template.Organization = "test-org"

	certDER, err := CreateCertificate(template, signer)
	if err != nil {
		t.Fatal(err)
	}
	if len(certDER) == 0 {
		t.Fatal("empty cert DER")
	}

	// Verify using our PQC verification
	pub, err := VerifyCertificatePQC(certDER)
	if err != nil {
		t.Fatal(err)
	}
	expectedPub := signer.Public().(*PublicKey)
	if !bytes.Equal(pub.Bytes, expectedPub.Bytes) {
		t.Error("certificate public key != signer public key")
	}
}

func TestParseCertificatePQC(t *testing.T) {
	_, secret, err := pqchelpereddsamldsaslhdsa.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	signer, err := NewHybridSigner(secret)
	if err != nil {
		t.Fatal(err)
	}
	template, _ := DefaultCertTemplate(time.Hour)
	certDER, err := CreateCertificate(template, signer)
	if err != nil {
		t.Fatal(err)
	}

	tbsDER, sig, pubBytes, err := ParseCertificatePQC(certDER)
	if err != nil {
		t.Fatal(err)
	}
	if len(tbsDER) == 0 || len(sig) == 0 || len(pubBytes) != PublicKeySize() {
		t.Errorf("parse: tbs=%d sig=%d pub=%d", len(tbsDER), len(sig), len(pubBytes))
	}
	expectPub := signer.Public().(*PublicKey)
	if !bytes.Equal(pubBytes, expectPub.Bytes) {
		t.Error("parsed public key != signer public key")
	}
	_ = tbsDER
}

func TestCertificatePEM(t *testing.T) {
	_, secret, _ := pqchelpereddsamldsaslhdsa.GenerateKey()
	signer, _ := NewHybridSigner(secret)
	template, _ := DefaultCertTemplate(time.Hour)
	template.SerialNumber = big.NewInt(1)
	certDER, err := CreateCertificate(template, signer)
	if err != nil {
		t.Fatal(err)
	}
	pem := CertificatePEM(certDER)
	if len(pem) == 0 || !bytes.Contains(pem, []byte("CERTIFICATE")) {
		t.Error("PEM should contain CERTIFICATE block")
	}
}

func TestDefaultCertTemplate(t *testing.T) {
	tmpl, err := DefaultCertTemplate(365 * 24 * time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if tmpl.SerialNumber == nil || tmpl.NotAfter.Before(tmpl.NotBefore) {
		t.Error("invalid default template")
	}
}

func TestCertNotAfter(t *testing.T) {
	_, secret, err := pqchelpereddsamldsaslhdsa.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	signer, err := NewHybridSigner(secret)
	if err != nil {
		t.Fatal(err)
	}
	validity := 24 * time.Hour
	template, err := DefaultCertTemplate(validity)
	if err != nil {
		t.Fatal(err)
	}
	certDER, err := CreateCertificate(template, signer)
	if err != nil {
		t.Fatal(err)
	}
	notAfter, err := CertNotAfter(certDER)
	if err != nil {
		t.Fatal(err)
	}
	if notAfter.Before(template.NotBefore) || notAfter.Sub(template.NotBefore) < validity-time.Second {
		t.Errorf("CertNotAfter: got %v, expected ~%v", notAfter, template.NotAfter)
	}
	// Invalid DER
	_, err = CertNotAfter([]byte{0x00})
	if err == nil {
		t.Error("CertNotAfter should fail on invalid DER")
	}
}


