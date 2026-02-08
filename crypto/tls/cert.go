package tls

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"math/big"
	"time"

	"github.com/quantumcoinproject/quantum-coin-go/common"
	"github.com/quantumcoinproject/quantum-coin-go/crypto"
	"github.com/quantumcoinproject/quantum-coin-go/crypto/cryptobase"
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
	oidCommonName   = asn1.ObjectIdentifier{2, 5, 4, 3}
	oidOrganization = asn1.ObjectIdentifier{2, 5, 4, 10}
)

// X.509 v3 extension OIDs (RFC 5280).
var (
	oidExtensionBasicConstraints = asn1.ObjectIdentifier{2, 5, 29, 19}
	oidExtensionKeyUsage         = asn1.ObjectIdentifier{2, 5, 29, 15}
	oidExtensionExtKeyUsage      = asn1.ObjectIdentifier{2, 5, 29, 37}
	oidKPServerAuth              = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 1}
	oidKPClientAuth              = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 2}
)

// validity is NotBefore and NotAfter (UTCTime).
type validity struct {
	NotBefore time.Time
	NotAfter  time.Time
}

// extension is Extension ::= SEQUENCE { extnID OID, critical BOOLEAN DEFAULT FALSE, extnValue OCTET STRING } (RFC 5280).
type extension struct {
	ExtnID    asn1.ObjectIdentifier
	Critical  bool `asn1:"optional"`
	ExtnValue []byte `asn1:"tag:4"` // OCTET STRING, holds DER-encoded value
}

// tbsCertificate is the TBSCertificate structure (X.509 v3 with extensions).
type tbsCertificate struct {
	Version            int `asn1:"optional,explicit,tag:0,default:0"`
	SerialNumber       *big.Int
	Signature          algorithmIdentifier
	Issuer             asn1.RawValue
	Validity           validity
	Subject            asn1.RawValue
	SubjectPublicKeyInfo subjectPublicKeyInfo
	Extensions         []extension `asn1:"optional,explicit,tag:3"`
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

// basicConstraintsValue is SEQUENCE { cA BOOLEAN DEFAULT FALSE, pathLenConstraint INTEGER OPTIONAL } (RFC 5280).
type basicConstraintsValue struct {
	CA bool `asn1:"optional"`
}

// buildExtensions returns X.509 v3 extensions: BasicConstraints (CA:FALSE), KeyUsage (digitalSignature, keyEncipherment), ExtKeyUsage (serverAuth, clientAuth).
func buildExtensions() ([]extension, error) {
	// BasicConstraints: cA = FALSE (end-entity cert)
	bcVal := basicConstraintsValue{CA: false}
	bcDER, err := asn1.Marshal(bcVal)
	if err != nil {
		return nil, err
	}
	// KeyUsage: digitalSignature (bit 0) + keyEncipherment (bit 2) per RFC 5280
	keyUsageDER, err := asn1.Marshal(asn1.BitString{Bytes: []byte{0x80 | 0x20}, BitLength: 8}) // one byte, bits 0 and 2 set
	if err != nil {
		return nil, err
	}
	// ExtKeyUsage: SEQUENCE OF OID { id-kp-serverAuth, id-kp-clientAuth }
	extKeyUsageDER, err := asn1.Marshal([]asn1.ObjectIdentifier{oidKPServerAuth, oidKPClientAuth})
	if err != nil {
		return nil, err
	}
	return []extension{
		{ExtnID: oidExtensionBasicConstraints, Critical: true, ExtnValue: bcDER},
		{ExtnID: oidExtensionKeyUsage, Critical: true, ExtnValue: keyUsageDER},
		{ExtnID: oidExtensionExtKeyUsage, ExtnValue: extKeyUsageDER},
	}, nil
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

	exts, err := buildExtensions()
	if err != nil {
		return nil, err
	}
	algo := algorithmIdentifier{Algorithm: oidHybridPQC}
	tbs := tbsCertificate{
		Version:            2, // v3 (required when extensions present)
		SerialNumber:       template.SerialNumber,
		Signature:          algo,
		Issuer:             issuer,
		Validity:           validity{NotBefore: template.NotBefore, NotAfter: template.NotAfter},
		Subject:            subject,
		SubjectPublicKeyInfo: subjectPublicKeyInfo{
			Algorithm: algo,
			PublicKey: asn1.BitString{Bytes: pub.Bytes, BitLength: len(pub.Bytes) * 8},
		},
		Extensions: exts,
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

// CertNotAfter returns the NotAfter (expiry) time of a PQC certificate (DER).
// Returns zero time and an error if the certificate cannot be parsed.
func CertNotAfter(certDER []byte) (time.Time, error) {
	var cert certificate
	rest, err := asn1.Unmarshal(certDER, &cert)
	if err != nil || len(rest) != 0 {
		return time.Time{}, errors.New("invalid certificate DER")
	}
	return cert.TBSCertificate.Validity.NotAfter, nil
}

// CertNotBefore returns the NotBefore time of a PQC certificate (DER).
func CertNotBefore(certDER []byte) (time.Time, error) {
	var cert certificate
	rest, err := asn1.Unmarshal(certDER, &cert)
	if err != nil || len(rest) != 0 {
		return time.Time{}, errors.New("invalid certificate DER")
	}
	return cert.TBSCertificate.Validity.NotBefore, nil
}

// CertSubjectNames parses the Subject of a PQC certificate and returns the CommonName and Organization.
func CertSubjectNames(certDER []byte) (commonName, organization string, err error) {
	var cert certificate
	rest, err := asn1.Unmarshal(certDER, &cert)
	if err != nil || len(rest) != 0 {
		return "", "", errors.New("invalid certificate DER")
	}
	var rdns []relativeDistinguishedNameSET
	_, err = asn1.Unmarshal(cert.TBSCertificate.Subject.Bytes, &rdns)
	if err != nil {
		return "", "", err
	}
	for _, set := range rdns {
		for _, attr := range set.Attributes {
			if len(attr.Type) == len(oidCommonName) {
				match := true
				for i := range oidCommonName {
					if attr.Type[i] != oidCommonName[i] {
						match = false
						break
					}
				}
				if match {
					commonName = attr.Value
				}
			}
			if len(attr.Type) == len(oidOrganization) {
				match := true
				for i := range oidOrganization {
					if attr.Type[i] != oidOrganization[i] {
						match = false
						break
					}
				}
				if match {
					organization = attr.Value
				}
			}
		}
	}
	return commonName, organization, nil
}

// CertInfo holds display metadata from a PQC certificate.
type CertInfo struct {
	CommonName   string
	Organization string
	NotBefore    time.Time
	NotAfter     time.Time
}

// CertMetadata returns the CN, O, NotBefore, and NotAfter of a PQC certificate (DER).
func CertMetadata(certDER []byte) (m CertInfo, err error) {
	m.NotBefore, err = CertNotBefore(certDER)
	if err != nil {
		return m, err
	}
	m.NotAfter, err = CertNotAfter(certDER)
	if err != nil {
		return m, err
	}
	m.CommonName, m.Organization, err = CertSubjectNames(certDER)
	return m, err
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

// VerifyCertificatePQC verifies the signature on a PQC certificate using cryptobase.DynamicVerifier.Verify.
// The cert stores the raw compact signature; we build the combined format (algId + sig, pubKey) expected by DynamicVerifier.
func VerifyCertificatePQC(certDER []byte) (pub *PublicKey, err error) {
	tbsDER, signature, publicKeyBytes, err := ParseCertificatePQC(certDER)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(tbsDER)
	// DynamicVerifier routes on part1[0]; compact.Verify strips a leading alg ID so we prepend it here.
	part1 := append([]byte{byte(crypto.MLDSA_ED25519_SLHDSA_COMPACT_ID)}, signature...)
	combinedSig := common.CombineTwoParts(part1, publicKeyBytes)
	if !cryptobase.DynamicSigVerifier.Verify(publicKeyBytes, hash[:], combinedSig) {
		return nil, errors.New("certificate signature verification failed")
	}
	return &PublicKey{Bytes: publicKeyBytes}, nil
}

// VerifyPeerCertificatesPQC verifies each certificate in rawCerts as a PQC cert (e.g. for use in tls.Config.VerifyPeerCertificate).
// Returns an error if rawCerts is empty or any cert fails verification.
func VerifyPeerCertificatesPQC(rawCerts [][]byte) error {
	if len(rawCerts) == 0 {
		return errors.New("no peer certificates")
	}
	for _, der := range rawCerts {
		if _, err := VerifyCertificatePQC(der); err != nil {
			return errors.New("peer certificate PQC verification failed: " + err.Error())
		}
	}
	return nil
}
