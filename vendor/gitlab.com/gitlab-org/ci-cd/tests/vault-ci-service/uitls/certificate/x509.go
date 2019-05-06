package certificate

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"time"

	"github.com/pkg/errors"
)

const (
	x509CertificatePrivateKeyBits = 2048
)

type Info struct {
	CertPEM       []byte
	PrivateKeyPEM []byte
}

type CA struct {
	serialNumber *big.Int
	caCert       Info
	signedCerts  map[string]Info
}

func NewCA() *CA {
	return &CA{
		serialNumber: big.NewInt(time.Now().Unix()),
		signedCerts:  make(map[string]Info, 0),
	}
}

func (ca *CA) CaCert() Info {
	return ca.caCert
}

func (ca *CA) SignedCerts() map[string]Info {
	return ca.signedCerts
}

func (ca *CA) Initialize() error {
	template := &x509.Certificate{
		SerialNumber: ca.increaseSerialNumber(),
		Subject: pkix.Name{
			CommonName: "Vault Test Service CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, x509CertificatePrivateKeyBits)
	if err != nil {
		return errors.Wrap(err, "couldn't generate CA private key")
	}

	certificate, err := ca.createCertificate(template, privateKey, template, privateKey)
	if err != nil {
		return errors.Wrap(err, "couldn't generate CA certificate")
	}

	ca.caCert = certificate

	return nil
}

func (ca *CA) increaseSerialNumber() *big.Int {
	ca.serialNumber.Add(ca.serialNumber, big.NewInt(1))

	return ca.serialNumber
}

func (ca *CA) createCertificate(certTemplate *x509.Certificate, privateKey *rsa.PrivateKey, caCert *x509.Certificate, caKey crypto.PrivateKey) (Info, error) {
	publicKeyBytes, err := x509.CreateCertificate(rand.Reader, certTemplate, caCert, privateKey.Public(), caKey)
	if err != nil {
		return Info{}, errors.Wrap(err, "couldn't create certificate")
	}

	certificate := Info{
		CertPEM:       pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: publicKeyBytes}),
		PrivateKeyPEM: pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}),
	}

	return certificate, nil
}

func (ca *CA) NewSignedCert(commonName string, ipAddress net.IP) (Info, error) {
	_, ok := ca.signedCerts[commonName]
	if ok {
		return ca.signedCerts[commonName], nil
	}

	template := &x509.Certificate{
		SerialNumber: ca.increaseSerialNumber(),
		Subject: pkix.Name{
			CommonName: commonName,
		},
		IPAddresses: []net.IP{ipAddress},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(1, 0, 0),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, x509CertificatePrivateKeyBits)
	if err != nil {
		return Info{}, errors.Wrap(err, "couldn't generate certificate private key")
	}

	caCertificate, caKey, err := ca.loadCaCertificate()
	if err != nil {
		return Info{}, errors.Wrap(err, "couldn't load CA certificate")
	}

	certificate, err := ca.createCertificate(template, privateKey, caCertificate, caKey)
	if err != nil {
		return Info{}, errors.Wrap(err, "couldn't generate signed certificate")
	}

	ca.signedCerts[commonName] = certificate

	return certificate, nil
}

func (ca *CA) loadCaCertificate() (*x509.Certificate, crypto.PrivateKey, error) {
	caTLS, err := tls.X509KeyPair(ca.caCert.CertPEM, ca.caCert.PrivateKeyPEM)
	if err != nil {
		return nil, nil, errors.Wrap(err, "couldn't generate CA certificate x509 key pair")
	}

	caCert, err := x509.ParseCertificate(caTLS.Certificate[0])
	if err != nil {
		return nil, nil, errors.Wrap(err, "couldn't parse CA certificate key pair")
	}

	return caCert, caTLS.PrivateKey, nil
}
