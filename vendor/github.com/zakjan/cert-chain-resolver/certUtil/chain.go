package certUtil

import (
	"crypto/x509"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

func isSelfSigned(cert *x509.Certificate) bool {
	return cert.CheckSignatureFrom(cert) == nil
}

func isChainRootNode(cert *x509.Certificate) bool {
	if isSelfSigned(cert) {
		return true
	}
	return false
}

func FetchCertificateChain(cert *x509.Certificate) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate

	certs = append(certs, cert)

MainLoop:
	for {
		certificate := certs[len(certs)-1]
		log := logrus.
			WithFields(logrus.Fields{
				"subject":       certificate.Subject.CommonName,
				"issuer":        certificate.Issuer.CommonName,
				"serial":        certificate.SerialNumber.String(),
				"issuerCertURL": certificate.IssuingCertificateURL,
				"dns":           certificate.DNSNames,
			})

		if certificate.IssuingCertificateURL != nil {
			parentURL := certificate.IssuingCertificateURL[0]

			log.Info("[cert verification] Requesting issure certificate")

			resp, err := http.Get(parentURL)
			if resp != nil {
				defer resp.Body.Close()
			}
			if err != nil {
				log.
					WithError(err).
					Error("[cert verification] Requesting issure certificate - HTTP request error")
				return nil, err
			}

			data, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.
					WithError(err).
					Error("[cert verification] Requesting issure certificate - response body read error")
				return nil, err
			}

			cert, err := DecodeCertificate(data)
			if err != nil {
				log.
					WithError(err).
					Error("[cert verification] Requesting issure certificate - certificate decoding error")
				return nil, err
			}

			log = log.
				WithFields(logrus.Fields{
					"newCert-subject":       cert.Subject.CommonName,
					"newCert-issuer":        cert.Issuer.CommonName,
					"newCert-serial":        cert.SerialNumber.String(),
					"newCert-issuerCertURL": cert.IssuingCertificateURL,
				})

			log.Info("[cert verification] Requesting issure certificate - appending the certificate to the chain")
			time.Sleep(1000 * time.Millisecond)

			certs = append(certs, cert)
			if isChainRootNode(cert) {
				log.Info("[cert verification] Requesting issure certificate - cert is a ROOT certificate so exiting the loop")
			}

		} else {
			log.Info("[cert verification] Certificate doesn't provide parent URL - exiting the loop")

			chains, err := certificate.Verify(x509.VerifyOptions{})
			if err != nil {
				if _, e := err.(x509.UnknownAuthorityError); e {
					continue
				}
				return nil, err
			}

			for _, cert := range chains[0] {
				if certificate.Equal(cert) {
					println("equal")
					break MainLoop
				}

				logrus.
					WithFields(logrus.Fields{
						"subject":       cert.Subject.CommonName,
						"issuer":        cert.Issuer.CommonName,
						"serial":        cert.SerialNumber.String(),
						"issuerCertURL": cert.IssuingCertificateURL,
					}).
					Info("[cert verification] Adding cert from verify chain to the final chain")

				certs = append(certs, cert)
			}
		}
	}

	return certs, nil
}

func AddRootCA(certs []*x509.Certificate) ([]*x509.Certificate, error) {
	lastCert := certs[len(certs)-1]

	logrus.
		WithFields(logrus.Fields{
			"subject":       lastCert.Subject.CommonName,
			"issuer":        lastCert.Issuer.CommonName,
			"serial":        lastCert.SerialNumber.String(),
			"issuerCertURL": lastCert.IssuingCertificateURL,
		}).
		Info("[cert verification] Verifying certificate")
	chains, err := lastCert.Verify(x509.VerifyOptions{})
	if err != nil {
		logrus.
			WithError(err).
			Error("[cert verification] AddRootCA() last certificate verification failure")
		if _, e := err.(x509.UnknownAuthorityError); e {
			return certs, nil
		}
		return nil, err
	}

	for _, cert := range chains[0] {
		logrus.
			WithFields(logrus.Fields{
				"subject":       cert.Subject.CommonName,
				"issuer":        cert.Issuer.CommonName,
				"serial":        cert.SerialNumber.String(),
				"issuerCertURL": cert.IssuingCertificateURL,
			}).
			Info("[cert verification] Adding cert from verify chain to the final chain")
		time.Sleep(1000 * time.Millisecond)

		certs = append(certs, cert)
	}

	return certs, nil
}
