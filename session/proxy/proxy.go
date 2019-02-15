package serviceproxy

import (
	"net/http"
	// "io"
	// "fmt"
)

type ProxyPool struct {
	Proxies map[string]*Proxy
}

type ProxyPooler interface {
	GetProxyPool() ProxyPool
}

type Proxy struct {
	*ProxySettings
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
}

type ProxyConn interface {
	ProxyRequest(w http.ResponseWriter, r *http.Request, buildOrService, requestedUri string, port int)
}

// stoppers is the number of goroutines that may attempt to call Stop()
func NewProxySettings(serviceName string, ports []ProxyPortSettings) *ProxySettings {
	service := ""
	if serviceName != "build" {
		service = serviceName
	}

	return &ProxySettings{
		ServiceName: service,
		Ports:       ports,
	}
}

// func (p *Proxy) ProxyRequest(w http.ResponseWriter, req *http.Request, buildOrService, requestedUri string) {
// 	// request := c.client.Get().
// 	//   Namespace(c.ns).
// 	//   Resource("services").
// 	//   SubResource("proxy").
// 	//   Name(net.JoinSchemeNamePort(scheme, name, port)).
// 	//   Suffix(path)
//
// 	fmt.Println("Proxeando")
// 	p.rewriteRequestURL(req, requestedUri)
//   resp, err := http.DefaultTransport.RoundTrip(req)
//   if err != nil {
//     http.Error(w, err.Error(), http.StatusServiceUnavailable)
//     return
//   }
//   defer resp.Body.Close()
//   p.copyHeader(w.Header(), resp.Header)
//   w.WriteHeader(resp.StatusCode)
//   io.Copy(w, resp.Body)
// }
//
// func (p *Proxy) copyHeader(dst, src http.Header) {
//   for k, vv := range src {
//     for _, v := range vv {
//       dst.Add(k, v)
//     }
//   }
// }
//
// func (p *Proxy) rewriteRequestURL(r *http.Request, requestedUri string) *http.Request {
// 	r.URL.Path = fmt.Sprintf("/%v", requestedUri)
// 	r.URL.Host = fmt.Sprintf("%v:%v", p.Host, p.Port)
//
// 	// Fallback to http ??
// 	if (r.URL.Scheme == "") {
// 		r.URL.Scheme = "http"
// 	}
//
// 	return r
// }
//
// func (p *Proxy) ProxyTunnel(w http.ResponseWriter, req *http.Request, buildOrService, requestedUri string) {
// 	fmt.Println("TUUUUUNEL")
// }
