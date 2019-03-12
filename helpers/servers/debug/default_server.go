package debug

import (
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
)

type serveMux interface {
	http.Handler
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

var newServeMux = func() serveMux {
	return http.NewServeMux()
}

type httpServer interface {
	Serve(listener net.Listener) error
}

var newHTTPServer = func(handler http.Handler) httpServer {
	return &http.Server{Handler: handler}
}

type prometheusRegistry interface {
	Register(collector prometheus.Collector) error
	Gather() ([]*dto.MetricFamily, error)
}

var newPrometheusRegistry = func() prometheusRegistry {
	return prometheus.NewRegistry()
}

type defaultServer struct {
	mux                serveMux
	prometheusRegistry prometheusRegistry
}

func newDefaultServer() (*defaultServer, error) {
	server := new(defaultServer)

	err := server.init()
	if err != nil {
		return nil, err
	}

	return server, nil
}

func (s *defaultServer) init() error {
	initializers := []func() error{
		s.initMux,
		s.initPrometheus,
		s.initPprof,
	}

	for _, initializer := range initializers {
		err := initializer()
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *defaultServer) initMux() error {
	s.mux = newServeMux()

	return nil
}

func (s *defaultServer) initPrometheus() error {
	s.prometheusRegistry = newPrometheusRegistry()
	s.mux.Handle("/metrics", promhttp.HandlerFor(s.prometheusRegistry, promhttp.HandlerOpts{}))

	collectors := CollectorsMap{
		"go stats":      prometheus.NewGoCollector(),
		"process stats": prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
	}

	return s.RegisterPrometheusCollectors(collectors)
}

func (s *defaultServer) RegisterPrometheusCollectors(collectors CollectorsMap) error {
	for name, collector := range collectors {
		err := s.RegisterPrometheusCollector(collector)
		if err != nil {
			return fmt.Errorf("error while registering %q Prometheus collector: %v", name, err)
		}
	}

	return nil
}

func (s *defaultServer) RegisterPrometheusCollector(collector prometheus.Collector) error {
	return s.prometheusRegistry.Register(collector)
}

func (s *defaultServer) initPprof() error {
	endpoints := EndpointsMap{
		"pprof/":        pprof.Index,
		"pprof/cmdline": pprof.Cmdline,
		"pprof/profile": pprof.Profile,
		"pprof/symbol":  pprof.Symbol,
		"pprof/trace":   pprof.Trace,
	}

	s.RegisterDebugEndpoints(endpoints)

	return nil
}

func (s *defaultServer) RegisterDebugEndpoints(endpoints EndpointsMap) {
	for path, endpoint := range endpoints {
		s.RegisterDebugEndpoint(path, endpoint)
	}
}

func (s *defaultServer) RegisterDebugEndpoint(path string, handlerFn http.HandlerFunc) {
	path = fmt.Sprintf("/debug/%s", strings.TrimLeft(path, "/"))
	s.mux.HandleFunc(path, handlerFn)
}

func (s *defaultServer) Start(listener net.Listener) error {
	return newHTTPServer(s.mux).Serve(listener)
}
