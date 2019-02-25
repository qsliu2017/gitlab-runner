package kubernetes

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
	serviceproxy "gitlab.com/gitlab-org/gitlab-runner/session/service_proxy"
	utils "gitlab.com/gitlab-org/gitlab-runner/utils"
	terminal "gitlab.com/gitlab-org/gitlab-terminal"
	errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8net "k8s.io/apimachinery/pkg/util/net"
	rest "k8s.io/client-go/rest"
)

func (s *executor) GetProxyPool() serviceproxy.ProxyPool {
	return s.ProxyPool
}

func (e *executor) ProxyWSRequest(w http.ResponseWriter, r *http.Request, requestedUri string, portSettings *serviceproxy.ProxyPortSettings, proxy *serviceproxy.ProxySettings) {
	// TODO: in order to avoid calling this method, and use one of its own,
	// we should refactor the library "gitlab.com/gitlab-org/gitlab-terminal"
	// and make it more generic, not so terminal focused, with a broader
	// terminology
	settings, err := e.getTerminalSettings()
	if err != nil {
		fmt.Errorf("Service proxy: error proxying")
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}

	req := e.serviceEndpointRequest(r.Method, proxy.ServiceName, requestedUri, portSettings)

	u := req.URL()
	u.Scheme = utils.WebsocketProtocolFor(u.Scheme)

	settings.Url = u.String()
	serviceProxy := terminal.NewWebSocketProxy(1)
	terminal.ProxyWebSocket(w, r, settings, serviceProxy)
}

func (e *executor) ProxyHTTPRequest(w http.ResponseWriter, r *http.Request, requestedUri string, portSettings *serviceproxy.ProxyPortSettings, proxy *serviceproxy.ProxySettings) {
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
	portSettings := proxy.PortSettingsFor(port)
	if portSettings == nil {
		fmt.Errorf("Port proxy %s not found", port)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	if !e.servicesRunning() {
		fmt.Errorf("Services are not ready yet")
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}

	if websocket.IsWebSocketUpgrade(r) {
		e.ProxyWSRequest(w, r, requestedUri, portSettings, proxy)
	} else {
		e.ProxyHTTPRequest(w, r, requestedUri, portSettings, proxy)
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

func (e *executor) servicesRunning() bool {
	pod, err := e.kubeClient.CoreV1().Pods(e.pod.Namespace).Get(e.pod.Name, metav1.GetOptions{})

	if err != nil || pod.Status.Phase != "Running" {
		return false
	}

	for _, container := range pod.Status.ContainerStatuses {
		if !container.Ready {
			return false
		}
	}

	return true
}
