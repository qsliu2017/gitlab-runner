package kubernetes

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
	serviceproxy "gitlab.com/gitlab-org/gitlab-runner/session/serviceproxy"
	terminal "gitlab.com/gitlab-org/gitlab-terminal"
	"k8s.io/apimachinery/pkg/api/errors"
	k8net "k8s.io/apimachinery/pkg/util/net"
	rest "k8s.io/client-go/rest"
)

func (s *executor) GetProxyPool() serviceproxy.ProxyPool {
	return s.ProxyPool
}

func (e *executor) ProxyWSRequest(w http.ResponseWriter, r *http.Request, requestedUri, port string, proxy *serviceproxy.ProxySettings) {
	portSettings := proxy.PortSettingsFor(port)
	if portSettings == nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	settings, err := e.getTerminalSettings()
	if err != nil {
		fmt.Errorf("Service proxy: error proxying")
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}

	req := e.serviceEndpointRequest(r.Method, proxy.ServiceName, requestedUri, portSettings)

	u := req.URL()
	u.Scheme = wsProtocolFor(u.Scheme)

	settings.Url = u.String()
	serviceProxy := terminal.NewWebSocketProxy(1)
	terminal.ProxyWebSocket(w, r, settings, serviceProxy)
}

func (e *executor) ProxyHTTPRequest(w http.ResponseWriter, r *http.Request, requestedUri, port string, proxy *serviceproxy.ProxySettings) {
	// if e.portForwarder == nil {
	// 	err := e.createPodPortForwards()
	// 	log.Printf("err: %#+v\n", err)
	// }
	portSettings := proxy.PortSettingsFor(port)
	if portSettings == nil {
		fmt.Errorf("Port proxy %s not found", port)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	body, err := e.serviceEndpointRequest(r.Method, proxy.ServiceName, requestedUri, portSettings).Stream()

	if err != nil {
		message, code := e.parseError(err)
		w.WriteHeader(code)

		if message != "" {
			fmt.Fprintf(w, message)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	io.Copy(w, body)
}

func (e *executor) ProxyRequest(w http.ResponseWriter, r *http.Request, requestedUri, port string, proxy *serviceproxy.ProxySettings) {
	if websocket.IsWebSocketUpgrade(r) {
		e.ProxyWSRequest(w, r, requestedUri, port, proxy)
	} else {
		e.ProxyHTTPRequest(w, r, requestedUri, port, proxy)
	}
}

func (e *executor) serviceEndpointRequest(verb, serviceName, requestedUri string, portSettings *serviceproxy.ProxyPortSettings) *rest.Request {
	return e.kubeClient.CoreV1().RESTClient().Verb(verb).
		Namespace(e.pod.Namespace).
		Resource("services").
		SubResource("proxy").
		Name(k8net.JoinSchemeNamePort(portSettings.Scheme(), serviceName, strconv.Itoa(portSettings.ExternalPort))).
		Suffix(requestedUri)
}

func (e *executor) newProxy(serviceName string, ports []serviceproxy.ProxyPortSettings) *serviceproxy.Proxy {
	return &serviceproxy.Proxy{
		Settings:          serviceproxy.NewProxySettings(serviceName, ports),
		ConnectionHandler: e,
	}
}

func (e *executor) parseError(err error) (string, int) {
	statusError, ok := err.(*errors.StatusError)

	if !ok {
		return "", http.StatusInternalServerError
	}

	code := int(statusError.Status().Code)
	// When the error is a 503 we don't want to give any information
	// coming from Kubernetes
	if code == http.StatusServiceUnavailable {
		return "", code
	}

	details := statusError.Status().Details
	if details == nil {
		return "", code
	}

	causes := details.Causes
	if len(causes) != 0 {
		return causes[0].Message, code
	}

	return "", code
}

func wsProtocolFor(httpProtocol string) string {
	if httpProtocol == "https" {
		return "wss"
	}

	return "ws"
}
