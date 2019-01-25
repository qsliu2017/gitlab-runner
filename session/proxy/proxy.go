package proxy

import (
	"net/http"
	"io"
)

type ProxySettings struct {
	Ports string
	BuildOrService string
	RequestedURI string
}

type Proxy struct {
	Host string
	Ports string
	BuildOrService string
}

// stoppers is the number of goroutines that may attempt to call Stop()
func NewProxy(host, port, buildOrService string) *Proxy {
	return &Proxy{
		Host: host,
		Ports:     port,
		BuildOrService: buildOrService,
	}
}

func (p *Proxy) ProxyRequest(w http.ResponseWriter, req *http.Request, requestedUri string) {
	p.rewriteRequestURL(req, requestedUri)
  resp, err := http.DefaultTransport.RoundTrip(req)
  if err != nil {
      http.Error(w, err.Error(), http.StatusServiceUnavailable)
      return
  }
  defer resp.Body.Close()
  p.copyHeader(w.Header(), resp.Header)
  w.WriteHeader(resp.StatusCode)
  io.Copy(w, resp.Body)
}

func (p *Proxy) copyHeader(dst, src http.Header) {
  for k, vv := range src {
    for _, v := range vv {
        dst.Add(k, v)
    }
  }
}

func (p *Proxy) rewriteRequestURL(r *http.Request, requestedUri string) *http.Request {
	r.Host = p.Host + ":" + p.Ports
	r.RequestURI = "/" + requestedUri
	r.URL.Path = r.RequestURI
	r.URL.Host = r.Host

	// Fallback to https ??
	if (r.URL.Scheme == "") {
		r.URL.Scheme = "https"
	}

	return r
}
