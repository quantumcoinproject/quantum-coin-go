package tls

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"math/big"
	"time"
)

// OID for hybrid PQC (Ed25519 + ML-DSA + SLH-DSA). Private/experimental.
var oidHybridPQC = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 54321, 1, 1}

// CertTemplate holds the fields used to build a self-signed X.509 certificate.
type CertTemplate struct {
	SerialNumber *big.Int
	NotBefore    time.Time
	NotAfter     time.Time
	CommonName   string
	Organization string
}

// DefaultCertTemplate returns a template with serial and validity set.
func DefaultCertTemplate(validity time.Duration) (*CertTemplate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	now := time.Now()
	return &CertTemplate{
		SerialNumber: serial,
		NotBefore:    now,
		NotAfter:     now.Add(validity),
		CommonName:   "pqc-tls",
		Organization: "quantum-coin",
	}, nil
}

// algorithmIdentifier for SubjectPublicKeyInfo and Certificate signatureAlgorithm.
type algorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue `asn1:"optional"`
}

// subjectPublicKeyInfo for hybrid PQC: algorithm OID + BIT STRING (public key bytes).
type subjectPublicKeyInfo struct {
	Algorithm algorithmIdentifier
	PublicKey asn1.BitString
}

// rdnAttribute is a single AttributeTypeAndValue (OID + value).
type rdnAttribute struct {
	Type  asn1.ObjectIdentifier
	Value string
}

// name is a simplified X.501 Name: SEQUENCE of SET of SEQUENCE { type, value }.
// We use one RDN with CN and O for simplicity.
var (
	oidCommonName     = asn1.ObjectIdentifier{2, 5, 4, 3}
	oidOrganization   = asn1.ObjectIdentifier{2, 5, 4, 10}
)

// validity is NotBefore and NotAfter (UTCTime).
type validity struct {
	NotBefore time.Time
	NotAfter  time.Time
}

// tbsCertificate is the TBSCertificate structure (minimal set of fields).
type tbsCertificate struct {
	Version            int `asn1:"optional,explicit,tag:0,default:0"`
	SerialNumber       *big.Int
	Signature          algorithmIdentifier
	Issuer             asn1.RawValue
	Validity           validity
	Subject            asn1.RawValue
	SubjectPublicKeyInfo subjectPublicKeyInfo
}

// certificate is Certificate ::= SEQUENCE { tbsCertificate, signatureAlgorithm, signature }
type certificate struct {
	TBSCertificate     tbsCertificate
	SignatureAlgorithm algorithmIdentifier
	SignatureValue     asn1.BitString
}

// relativeDistinguishedNameSET is SET OF AttributeTypeAndValue (for one RDN).
type relativeDistinguishedNameSET struct {
	Attributes []rdnAttribute `asn1:"set"`
}

// buildName builds a minimal Name (SEQUENCE OF SET OF AttributeTypeAndValue) for issuer/subject.
func buildName(commonName, organization string) (asn1.RawValue, error) {
	// One RDN with two attributes: CN and O.
	rdn := []relativeDistinguishedNameSET{{
		Attributes: []rdnAttribute{
			{Type: oidCommonName, Value: commonName},
			{Type: oidOrganization, Value: organization},
		},
	}}
	nameBytes, err := asn1.Marshal(rdn)
	if err != nil {
		return asn1.RawValue{}, err
	}
	return asn1.RawValue{Class: 0, Tag: 16, IsCompound: true, Bytes: nameBytes}, nil
}

// CreateCertificate builds a self-signed X.509 certificate (DER) for the hybrid public key,
// signed by signer. Template defines serial, validity, and subject/issuer name.
func CreateCertificate(template *CertTemplate, signer *HybridSigner) (certDER []byte, err error) {
	if template == nil || signer == nil {
		return nil, errors.New("template and signer required")
	}
	pub := signer.Public().(*PublicKey)
	if len(pub.Bytes) != PublicKeySize() {
		return nil, errors.New("invalid public key size")
	}

	issuer, err := buildName(template.CommonName, template.Organization)
	if err != nil {
		return nil, err
	}
	subject, err := buildName(template.CommonName, template.Organization)
	if err != nil {
		return nil, err
	}

	algo := algorithmIdentifier{Algorithm: oidHybridPQC}
	tbs := tbsCertificate{
		Version:            0, // v1
		SerialNumber:       template.SerialNumber,
		Signature:          algo,
		Issuer:             issuer,
		Validity:           validity{NotBefore: template.NotBefore, NotAfter: template.NotAfter},
		Subject:            subject,
		SubjectPublicKeyInfo: subjectPublicKeyInfo{
			Algorithm: algo,
			PublicKey: asn1.BitString{Bytes: pub.Bytes, BitLength: len(pub.Bytes) * 8},
		},
	}

	tbsDER, err := asn1.Marshal(tbs)
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256(tbsDER)
	sig, err := signer.Sign(rand.Reader, hash[:], nil)
	if err != nil {
		return nil, err
	}

	cert := certificate{
		TBSCertificate:     tbs,
		SignatureAlgorithm: algo,
		SignatureValue:     asn1.BitString{Bytes: sig, BitLength: len(sig) * 8},
	}
	return asn1.Marshal(cert)
}

// CertificatePEM encodes cert DER as a PEM block.
func CertificatePEM(certDER []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
}

// PEMToCertDER decodes PEM and returns the first CERTIFICATE block's DER bytes.
func PEMToCertDER(pemBytes []byte) ([]byte, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("no CERTIFICATE block in PEM")
	}
	return block.Bytes, nil
}

// ParseCertificatePQC parses a PQC X.509 certificate (DER) and returns the raw TBSCertificate
// hash, signature, and hybrid public key bytes. It does not verify the signature; use VerifyCertificatePQC for that.
func ParseCertificatePQC(certDER []byte) (tbsDER []byte, signature []byte, publicKey []byte, err error) {
	var cert certificate
	rest, err := asn1.Unmarshal(certDER, &cert)
	if err != nil || len(rest) != 0 {
		return nil, nil, nil, errors.New("invalid certificate DER")
	}
	// Check algorithm is our OID
	if len(cert.SignatureAlgorithm.Algorithm) != len(oidHybridPQC) {
		return nil, nil, nil, errors.New("certificate not PQC")
	}
	for i := range oidHybridPQC {
		if cert.SignatureAlgorithm.Algorithm[i] != oidHybridPQC[i] {
			return nil, nil, nil, errors.New("certificate not PQC")
		}
	}
	tbsDER, err = asn1.Marshal(cert.TBSCertificate)
	if err != nil {
		return nil, nil, nil, err
	}
	publicKey = cert.TBSCertificate.SubjectPublicKeyInfo.PublicKey.RightAlign()
	if len(publicKey) != PublicKeySize() {
		return nil, nil, nil, errors.New("invalid PQC public key size in certificate")
	}
	signature = cert.SignatureValue.RightAlign()
	return tbsDER, signature, publicKey, nil
}

// VerifyCertificatePQC verifies the signature on a PQC certificate using the hybrid Verify.
func VerifyCertificatePQC(certDER []byte) (pub *PublicKey, err error) {
	tbsDER, signature, publicKeyBytes, err := ParseCertificatePQC(certDER)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(tbsDER)
	if !Verify(&PublicKey{Bytes: publicKeyBytes}, hash[:], signature) {
		return nil, errors.New("certificate signature verification failed")
	}
	return &PublicKey{Bytes: publicKeyBytes}, nil
}
