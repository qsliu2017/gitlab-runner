package vault

import (
	"fmt"
	"net"
	"net/url"
	"sort"

	"github.com/hashicorp/vault/api"
	"github.com/pkg/errors"

	"gitlab.com/gitlab-org/ci-cd/tests/vault-ci-service/uitls/certificate"
)

type authMethod string

const (
	Userpass authMethod = "userpass"
	TLSCert  authMethod = "cert"
)

const (
	DefaultReaderPolicy = "default-reader"
)

const (
	SecretTypeKV1 = "kv1"
	SecretTypeKV2 = "kv2"
)

type Details struct {
	URL       *url.URL
	RootToken string

	CA CADetails

	AuthMethods AuthMethodsDetails

	TestSecrets []TestSecret
}

type CADetails struct {
	CACert       TLSCertInfo
	Certificates map[string]TLSCertInfo
}

type TLSCertInfo struct {
	CertificatePEM string
	PrivateKeyPEM  string
}

type AuthMethodsDetails struct {
	Userpass UserpassDetails
	TLSCert  TLSCertDetails
}

type UserpassDetails struct {
	Username string
	Password string
}

type TLSCertDetails struct {
	CA       TLSCertInfo
	AuthCert TLSCertInfo
}

type TestSecret struct {
	Type string
	Path string
	Data map[string]interface{}
}

type Initializer struct {
	client *api.Client

	ca       *certificate.CA
	vaultURL *url.URL
}

func NewInitializer(vaultURL *url.URL, ca *certificate.CA) *Initializer {
	i := &Initializer{
		ca:       ca,
		vaultURL: vaultURL,
	}

	return i
}

func (i *Initializer) Initialize() (Details, error) {
	details := Details{
		URL: i.vaultURL,
	}

	vaultCli, err := i.getClient()
	if err != nil {
		return details, errors.Wrap(err, "couldn't create vault client")
	}

	initResp, err := i.initVault()
	if err != nil {
		return details, errors.Wrap(err, "couldn't initialize vault")
	}

	err = i.unsealVault(initResp)
	if err != nil {
		return details, errors.Wrapf(err, "couldn't unseal vault")
	}

	vaultCli.SetToken(initResp.RootToken)
	details.RootToken = initResp.RootToken

	err = i.setupDefaultReaderPolicy()
	if err != nil {
		return details, errors.Wrapf(err, "couldn't setup default reader policy")
	}

	authMethods := map[authMethod]func(details *Details) error{
		Userpass: i.setupUserpassAuth,
		TLSCert:  i.setupTLSCertAuth,
	}

	for method, setup := range authMethods {
		err = setup(&details)
		if err != nil {
			return details, errors.Wrapf(err, "couldn't setup %q auth method", method)
		}
	}

	secretsEngines := map[string]func(details *Details) error{
		"kv1": i.setupKV1TestSecret,
		"kv2": i.setupKV2TestSecret,
	}

	enginesList := make([]string, 0)
	for secretsEngine := range secretsEngines {
		enginesList = append(enginesList, secretsEngine)
	}

	sort.Strings(enginesList)

	for _, secretsEngine := range enginesList {
		err = secretsEngines[secretsEngine](&details)
		if err != nil {
			return details, errors.Wrapf(err, "couldn't setup test secrets for %q engine", secretsEngine)
		}
	}

	details.CA = CADetails{
		CACert: TLSCertInfo{
			CertificatePEM: string(i.ca.CaCert().CertPEM),
			PrivateKeyPEM:  string(i.ca.CaCert().PrivateKeyPEM),
		},
		Certificates: make(map[string]TLSCertInfo, 0),
	}
	for certName, cert := range i.ca.SignedCerts() {
		details.CA.Certificates[certName] = TLSCertInfo{
			CertificatePEM: string(cert.CertPEM),
			PrivateKeyPEM:  string(cert.PrivateKeyPEM),
		}
	}

	return details, nil
}

func (i *Initializer) getClient() (*api.Client, error) {
	if i.client != nil {
		return i.client, nil
	}

	vaultCliConfig := api.DefaultConfig()
	vaultCliConfig.Address = i.vaultURL.String()

	tlsConfig := &api.TLSConfig{
		Insecure: true,
	}

	err := vaultCliConfig.ConfigureTLS(tlsConfig)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't prepare TLS configuration for the new Vault client")
	}

	cli, err := api.NewClient(vaultCliConfig)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create new Vault client")
	}

	i.client = cli

	return cli, nil
}

func (i *Initializer) initVault() (*api.InitResponse, error) {
	initOpts := &api.InitRequest{
		SecretShares:    1,
		SecretThreshold: 1,
	}

	cli, err := i.getClient()
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create vault client")
	}

	return cli.Sys().Init(initOpts)
}

func (i *Initializer) unsealVault(resp *api.InitResponse) error {
	cli, err := i.getClient()
	if err != nil {
		return errors.Wrap(err, "couldn't create vault client")
	}

	for keyNumber, key := range resp.Keys {
		_, err := cli.Sys().Unseal(key)
		if err != nil {
			return errors.Wrapf(err, "failed on key %d", keyNumber)
		}
	}

	return nil
}

func (i *Initializer) setupDefaultReaderPolicy() error {
	cli, err := i.getClient()
	if err != nil {
		return errors.Wrap(err, "couldn't create vault client")
	}

	policy := `
path "kv1/*" {
  capabilities = ["read", "list"]
}

path "kv2/data/*" {
  capabilities = ["read", "list"]
}
`

	err = cli.Sys().PutPolicy(DefaultReaderPolicy, policy)
	if err != nil {
		return errors.Wrap(err, "couldn't write default-reader policy")
	}

	return nil
}

func (i *Initializer) setupUserpassAuth(details *Details) error {
	cli, err := i.getClient()
	if err != nil {
		return errors.Wrap(err, "couldn't create vault client")
	}

	username := "testuser"
	password := "testpass"

	authOptions := &api.EnableAuthOptions{
		Type: "userpass",
	}

	err = cli.Sys().EnableAuthWithOptions("userpass", authOptions)
	if err != nil {
		return errors.Wrap(err, "couldn't enable vault authorization method")
	}

	data := map[string]interface{}{
		"password": password,
		"policies": fmt.Sprintf("default,%s", DefaultReaderPolicy),
	}
	_, err = cli.Logical().Write(fmt.Sprintf("auth/userpass/users/%s", username), data)
	if err != nil {
		return errors.Wrapf(err, "couldn't create vault username %q", username)
	}

	details.AuthMethods.Userpass = UserpassDetails{
		Username: username,
		Password: password,
	}

	return nil
}

func (i *Initializer) setupTLSCertAuth(details *Details) error {
	cli, err := i.getClient()
	if err != nil {
		return errors.Wrap(err, "couldn't create vault client")
	}

	authOptions := &api.EnableAuthOptions{
		Type: "cert",
	}

	err = cli.Sys().EnableAuthWithOptions("cert", authOptions)
	if err != nil {
		return errors.Wrap(err, "couldn't enable vault authorization method")
	}

	caCert := i.ca.CaCert()
	signedCert, err := i.ca.NewSignedCert("cert1", net.ParseIP("127.0.0.1"))
	if err != nil {
		return errors.Wrap(err, "couldn't create signed certificate")
	}

	roleName := "cert1"
	data := map[string]interface{}{
		"certificate": string(caCert.CertPEM),
		"policies":    fmt.Sprintf("default,%s", DefaultReaderPolicy),
	}
	_, err = cli.Logical().Write(fmt.Sprintf("auth/cert/certs/%s", roleName), data)
	if err != nil {
		return errors.Wrapf(err, "couldn't create vault certificate %q", roleName)
	}

	details.AuthMethods.TLSCert = TLSCertDetails{
		CA: TLSCertInfo{
			CertificatePEM: string(caCert.CertPEM),
			PrivateKeyPEM:  string(caCert.PrivateKeyPEM),
		},
		AuthCert: TLSCertInfo{
			CertificatePEM: string(signedCert.CertPEM),
			PrivateKeyPEM:  string(signedCert.PrivateKeyPEM),
		},
	}

	return nil
}

func (i *Initializer) setupKV1TestSecret(details *Details) error {
	cli, err := i.getClient()
	if err != nil {
		return errors.Wrap(err, "couldn't create vault client")
	}

	kvMount := &api.MountInput{
		Type: "kv",
	}

	err = cli.Sys().Mount("kv1", kvMount)
	if err != nil {
		return errors.Wrap(err, "couldn't mount secrets engine")
	}

	data := map[string]interface{}{
		"keyString": "value1",
		"keyInt":    2,
		"keyBool":   false,
	}

	secretPath := "kv1/my-secrets"

	_, err = cli.Logical().Write(secretPath, data)
	if err != nil {
		return errors.Wrapf(err, "couldn't write secret at %q path", secretPath)
	}

	details.TestSecrets = append(details.TestSecrets, TestSecret{
		Type: SecretTypeKV1,
		Path: secretPath,
		Data: data,
	})

	return nil
}

func (i *Initializer) setupKV2TestSecret(details *Details) error {
	cli, err := i.getClient()
	if err != nil {
		return errors.Wrap(err, "couldn't create vault client")
	}

	kvMount := &api.MountInput{
		Type: "kv",
		Options: map[string]string{
			"version": "2",
		},
	}

	err = cli.Sys().Mount("kv2", kvMount)
	if err != nil {
		return errors.Wrap(err, "couldn't mount secrets engine")
	}

	data := map[string]interface{}{
		"data": map[string]interface{}{
			"keyString": "value1",
			"keyInt":    2,
			"keyBool":   false,
		},
	}

	secretPath := "kv2/data/my-secrets"

	_, err = cli.Logical().Write(secretPath, data)
	if err != nil {
		return errors.Wrapf(err, "couldn't write secret at %q path", secretPath)
	}

	details.TestSecrets = append(details.TestSecrets, TestSecret{
		Type: SecretTypeKV2,
		Path: secretPath,
		Data: data,
	})

	return nil
}
