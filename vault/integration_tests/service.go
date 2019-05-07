package integration_tests

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/ci-cd/tests/vault-ci-service/vault"

	"gitlab.com/gitlab-org/gitlab-runner/vault/config"
)

const (
	ServiceProxyPort  = 8443
	ServiceDirectPort = 8200
)

type Service struct {
	t *testing.T
}

func NewService(t *testing.T) *Service {
	return &Service{
		t: t,
	}
}

func (s *Service) getVaultHostname() string {
	hostname := os.Getenv("VAULT_HOSTNAME")
	if hostname != "" {
		return hostname
	}

	return "127.0.0.1"
}

func (s *Service) getBaseURL(port int) string {
	return fmt.Sprintf("https://%s:%d", s.getVaultHostname(), port)
}

func (s *Service) ReadMetadata() vault.Details {
	cli := http.DefaultClient
	cli.Transport = http.DefaultTransport
	cli.Transport.(*http.Transport).TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	resp, err := cli.Get(fmt.Sprintf("%s/metadata", s.getBaseURL(ServiceProxyPort)))
	require.NoError(s.t, err)

	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(s.t, err)

	var details vault.Details
	err = json.Unmarshal(body, &details)
	require.NoError(s.t, err)

	return details
}

func (s *Service) GetVaultServerConfig(port int) config.VaultServer {
	details := s.ReadMetadata()

	tlsCaFile, err := ioutil.TempFile("", "ca.cert")
	require.NoError(s.t, err)
	defer tlsCaFile.Close()

	_, err = io.Copy(tlsCaFile, bytes.NewBufferString(details.CA.CACert.CertificatePEM))
	require.NoError(s.t, err)

	err = tlsCaFile.Close()
	require.NoError(s.t, err)

	return config.VaultServer{
		URL:       s.getBaseURL(port),
		TLSCAFile: tlsCaFile.Name(),
	}
}

func (s *Service) GetVaultTokenAuthConfig() *config.VaultTokenAuth {
	details := s.ReadMetadata()

	return &config.VaultTokenAuth{
		Token: details.RootToken,
	}
}

func (s *Service) GetVaultUserpassAuthConfig() *config.VaultUserpassAuth {
	details := s.ReadMetadata()

	return &config.VaultUserpassAuth{
		Username: details.AuthMethods.Userpass.Username,
		Password: details.AuthMethods.Userpass.Password,
	}
}

func (s *Service) GetVaultTLSAuthConfig() *config.VaultTLSAuth {
	details := s.ReadMetadata()

	certFile, err := createTLSFile("client.crt", details.AuthMethods.TLSCert.AuthCert.CertificatePEM)
	require.NoError(s.t, err)

	keyFile, err := createTLSFile("client.key", details.AuthMethods.TLSCert.AuthCert.PrivateKeyPEM)
	require.NoError(s.t, err)

	return &config.VaultTLSAuth{
		TLSCertFile: certFile,
		TLSKeyFile:  keyFile,
	}
}

func createTLSFile(name string, data string) (string, error) {
	file, err := ioutil.TempFile("", name)
	if err != nil {
		return "", errors.Wrap(err, "error while creating temporary file")
	}

	buf := bytes.NewBufferString(data)
	bufLen := buf.Len()

	n, err := io.Copy(file, buf)
	if err != nil {
		return "", errors.Wrapf(err, "error while writing to temporary file %q", file.Name())
	}

	if n != int64(bufLen) {
		return "", errors.Wrapf(err, "length of data written to %q doesn't equal to the length of provided data", file.Name())
	}

	return file.Name(), nil
}

func (s *Service) GetVaultSecretsConfig() config.VaultSecrets {
	details := s.ReadMetadata()
	cfg := make(config.VaultSecrets, len(details.TestSecrets))

	for testSecretID, testSecret := range details.TestSecrets {
		cfg[testSecretID] = &config.VaultSecret{
			Type: config.VaultSecretType(testSecret.Type),
			Path: testSecret.Path,
			Keys: make(config.VaultSecretKeys, len(testSecret.Data)),
		}

		i := 0
		for key := range testSecret.Data {
			cfg[testSecretID].Keys[i] = &config.VaultSecretKey{
				Key:     key,
				EnvName: fmt.Sprintf("%s_VARIABLE_%02d", testSecret.Type, i),
			}
			i++
		}
	}

	return cfg
}
