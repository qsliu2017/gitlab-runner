package proxy

import (
	"net/http"
)

type ProxySettings struct {
	Ports string
	BuildOrService string
	RequestedURI string
}

type Proxy interface {
	Connect() (Conn, error)
}

type Conn interface {
	Start(w http.ResponseWriter, r *http.Request, buildOrService, port, requestedUri string)
	Close() error
}
