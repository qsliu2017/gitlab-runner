package config

import (
	"bytes"
	"io"
	"io/ioutil"
	"net"
	"os"
	"text/template"

	"github.com/pkg/errors"

	"gitlab.com/gitlab-org/ci-cd/tests/vault-ci-service/uitls/certificate"
)

const VaultConfigFilePattern = "vault-config"

var configTemplate = `
storage "inmem" {}

listener "tcp" {
  address = "0.0.0.0:8200"
  tls_disable = 0
  tls_cert_file = "{{.TLSCertFile}}"
  tls_key_file = "{{.TLSKeyFile}}"
  tls_client_ca_file = "{{.TLSClientCAFile}}"
}

ui = true
max_lease_ttl = "10h"
default_lease_ttl = "10h"
disable_mlock = true
`

type configDetails struct {
	TLSCertFile     string
	TLSKeyFile      string
	TLSClientCAFile string
}

func CreateFile(ca *certificate.CA) (string, error) {
	confFile, err := ioutil.TempFile("", VaultConfigFilePattern)
	if err != nil {
		return "", errors.Wrap(err, "couldn't create temporary config file")
	}

	configContent, err := prepareConfigContent(ca)
	if err != nil {
		return "", errors.Wrap(err, "couldn't prepare configuration content")
	}

	_, err = io.Copy(confFile, bytes.NewReader([]byte(configContent)))
	if err != nil {
		return "", errors.Wrap(err, "couldn't write temporary config file")
	}

	err = confFile.Close()
	if err != nil {
		return "", errors.Wrap(err, "couldn't close temporary config file")
	}

	err = os.Chmod(confFile.Name(), 0777)
	if err != nil {
		return "", errors.Wrap(err, "couldn't set permission for temporary config file")
	}

	return confFile.Name(), nil
}

func prepareConfigContent(ca *certificate.CA) (string, error) {
	cert, err := ca.NewSignedCert("vault", net.ParseIP("127.0.0.1"))
	if err != nil {
		return "", errors.Wrap(err, "couldn't create TLS certificate")
	}

	certFile, err := saveCertFile("vault.cert", cert.CertPEM)
	if err != nil {
		return "", errors.Wrap(err, "couldn't create certificate file")
	}

	keyFile, err := saveCertFile("vault.cert", cert.PrivateKeyPEM)
	if err != nil {
		return "", errors.Wrap(err, "couldn't create private key file")
	}

	caCert := ca.CaCert()

	caCertFile, err := saveCertFile("ca.cert", caCert.CertPEM)
	if err != nil {
		return "", errors.Wrap(err, "couldn't create CA certificate key file")
	}

	cfg := configDetails{
		TLSCertFile:     certFile,
		TLSKeyFile:      keyFile,
		TLSClientCAFile: caCertFile,
	}

	tpl, err := template.New("vault-config").Parse(configTemplate)
	if err != nil {
		return "", errors.Wrap(err, "couldn't parse configuration content template")
	}

	out := new(bytes.Buffer)
	err = tpl.Execute(out, &cfg)
	if err != nil {
		return "", errors.Wrap(err, "couldn't execute configuration content template")
	}

	return out.String(), nil
}

func saveCertFile(name string, data []byte) (string, error) {
	file, err := ioutil.TempFile("", name)
	if err != nil {
		return "", errors.Wrapf(err, "couldn't create temporary file for %q", name)
	}

	n, err := io.Copy(file, bytes.NewBuffer(data))
	if err != nil {
		return "", errors.Wrapf(err, "couldn't write to temporary file for %q", name)
	}

	if n != int64(len(data)) {
		return "", errors.Wrapf(err, "written size doesn't match data size for %q", name)
	}

	err = file.Close()
	if err != nil {
		return "", errors.Wrapf(err, "couldn't close temporary cert file for %q", name)
	}

	return file.Name(), nil
}
