package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"

	"gitlab.com/gitlab-org/ci-cd/tests/vault-ci-service/log"
	"gitlab.com/gitlab-org/ci-cd/tests/vault-ci-service/uitls/certificate"
	"gitlab.com/gitlab-org/ci-cd/tests/vault-ci-service/vault"
)

type Proxy struct {
	logger  log.Logger
	ca      *certificate.CA
	details vault.Details

	server   *http.Server
	handler  http.Handler
	listener net.Listener
}

func New(logger log.Logger, details vault.Details, ca *certificate.CA) (*Proxy, error) {
	p := &Proxy{
		logger:  logger,
		ca:      ca,
		details: details,
	}

	cert, err := ca.NewSignedCert("vault", net.ParseIP("127.0.0.1"))
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create TLS certificate")
	}

	tlsCert, err := tls.X509KeyPair(cert.CertPEM, cert.PrivateKeyPEM)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create TLS certificate/key pair")
	}

	caCert := p.ca.CaCert()
	pool := x509.NewCertPool()
	ok := pool.AppendCertsFromPEM(caCert.CertPEM)
	if !ok {
		return nil, errors.Wrap(err, "couldn't add CA certificate to ClientCAa pool")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		ClientAuth:   tls.VerifyClientCertIfGiven,
		ClientCAs:    pool,
	}

	listener, err := tls.Listen("tcp", "0.0.0.0:8443", tlsConfig)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create listener")
	}

	p.listener = listener

	router := mux.NewRouter()
	router.Path("/metadata").HandlerFunc(p.metadata)
	router.PathPrefix("/").HandlerFunc(p.vaultReverseProxy)

	p.handler = router

	return p, nil
}

func (p *Proxy) metadata(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	data, err := json.Marshal(p.details)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"error":"%s"}`, strings.Replace(err.Error(), `"`, `\\"`, -1))))
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (p *Proxy) vaultReverseProxy(w http.ResponseWriter, r *http.Request) {
	vaultURL := p.details.URL
	reverseProxy := httputil.NewSingleHostReverseProxy(vaultURL)

	caCert := p.ca.CaCert()
	pool := x509.NewCertPool()
	ok := pool.AppendCertsFromPEM(caCert.CertPEM)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
	}

	transport := http.DefaultTransport.(*http.Transport)
	transport.TLSClientConfig = &tls.Config{
		MinVersion: tls.VersionTLS10,
		RootCAs:    pool,
	}

	reverseProxy.Transport = transport

	r.URL.Host = vaultURL.Host
	r.URL.Scheme = vaultURL.Scheme
	r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	r.Host = vaultURL.Host

	p.logger.
		WithField("X-Vault-Token", r.Header.Get("X-Vault-Token")).
		WithField("path", r.URL.Path).
		Debug("Proxying Vault request")

	reverseProxy.ServeHTTP(w, r)
}

func (p *Proxy) Start() (net.Listener, chan error) {
	errCh := make(chan error, 1)

	go func() {
		p.server = &http.Server{Handler: p.handler}
		errCh <- p.server.Serve(p.listener)
	}()

	return p.listener, errCh
}
