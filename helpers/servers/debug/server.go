package debug

import (
	"net"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

type CollectorsMap map[string]prometheus.Collector

type EndpointsMap map[string]http.HandlerFunc

type Server interface {
	Start(listener net.Listener) error
	RegisterPrometheusCollectors(collectors CollectorsMap) error
	RegisterPrometheusCollector(collector prometheus.Collector) error
	RegisterDebugEndpoints(endpoints EndpointsMap)
	RegisterDebugEndpoint(path string, handlerFn http.HandlerFunc)
}

func NewServer() (Server, error) {
	return newDefaultServer()
}
