package proxy

import (
	"net/http"
	"io"
	"fmt"
)

type ProxyPool struct {
	Proxies map[int]*Proxy
}

type ProxyPooler interface {
	GetProxyPool() ProxyPool
}

type Proxy struct {
	Host string
	Port int
	BuildOrService string
}

// stoppers is the number of goroutines that may attempt to call Stop()
func NewProxy(host string, port int, buildOrService string) *Proxy {
	return &Proxy{
		Host: host,
		Port: port,
		BuildOrService: buildOrService,
	}
}

func (p *Proxy) ProxyRequest(w http.ResponseWriter, req *http.Request, buildOrService, requestedUri string) {
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
	r.URL.Path = fmt.Sprintf("/%v", requestedUri)
	r.URL.Host = fmt.Sprintf("%v:%v", p.Host, p.Port)

	// Fallback to http ??
	if (r.URL.Scheme == "") {
		r.URL.Scheme = "http"
	}

	return r
}
