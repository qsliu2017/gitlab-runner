package instance

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"time"
)

var serialNumberLimit = new(big.Int).Lsh(big.NewInt(1), 128)

type credentials struct {
	key    []byte
	ca     []byte
	server []byte
	client []byte
}

// generateCertificates generates key, ca, client and server certificates.
//
// Because these a single-user certificates, for the lifecycle of an instance,
// and not used outside of this context, the key is shared between each
// certificate.
func generateCertificates(hosts []string) (*credentials, error) {
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}

	base := func(usage x509.KeyUsage) *x509.Certificate {
		return &x509.Certificate{
			SerialNumber: serialNumber,
			Subject: pkix.Name{
				Organization: []string{"gitlab-runner"},
			},
			NotBefore:             time.Now().Add(-5 * time.Minute),
			NotAfter:              time.Now().Add(time.Hour * 24 * 365 * 3),
			BasicConstraintsValid: true,
			KeyUsage:              usage,
		}
	}

	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}

	parent := base(x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement | x509.KeyUsageCertSign)
	parent.IsCA = true
	ca, err := x509.CreateCertificate(rand.Reader, parent, parent, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}

	parent, err = x509.ParseCertificate(ca)
	if err != nil {
		return nil, err
	}

	clientCert := base(x509.KeyUsageDigitalSignature)
	clientCert.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	client, err := x509.CreateCertificate(rand.Reader, clientCert, parent, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}

	serverCert := base(x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement)
	serverCert.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			serverCert.IPAddresses = append(serverCert.IPAddresses, ip)
		} else {
			serverCert.DNSNames = append(serverCert.DNSNames, h)
		}
	}
	server, err := x509.CreateCertificate(rand.Reader, serverCert, parent, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}

	return &credentials{
		key:    pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}),
		ca:     pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca}),
		client: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: client}),
		server: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server}),
	}, nil
}
