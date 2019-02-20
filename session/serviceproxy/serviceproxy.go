package serviceproxy

import (
	"net/http"
	"strconv"
)

type ProxyPool struct {
	Proxies map[string]*Proxy
}

type ProxyPooler interface {
	GetProxyPool() ProxyPool
}

type Proxy struct {
	Settings          *ProxySettings
	ConnectionHandler ProxyConn
}

type ProxySettings struct {
	ServiceName string
	Ports       []ProxyPortSettings
}

type ProxyPortSettings struct {
	ExternalPort int
	InternalPort int
	SslEnabled   bool
	Name         string
}

type ProxyConn interface {
	ProxyRequest(w http.ResponseWriter, r *http.Request, requestedUri, port string, proxy *ProxySettings)
}

func NewProxySettings(serviceName string, ports []ProxyPortSettings) *ProxySettings {
	return &ProxySettings{
		ServiceName: serviceName,
		Ports:       ports,
	}
}

func (p *ProxySettings) PortSettingsFor(portNameOrNumber string) *ProxyPortSettings {
	intPort, _ := strconv.Atoi(portNameOrNumber)

	for _, port := range p.Ports {
		if port.ExternalPort == intPort || port.Name == portNameOrNumber {
			return &port
		}
	}

	return nil
}

func (p *ProxyPortSettings) Scheme() string {
	if p.SslEnabled == true {
		return "https"
	} else {
		return "http"
	}
}
