package certificate

//go:generate mockery --inpackage --name Generator

import "crypto/tls"

type Generator interface {
	Generate(host string) (tls.Certificate, []byte, error)
}
