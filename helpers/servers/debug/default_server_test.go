package debug

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type fakeListener struct{}

func (l *fakeListener) Accept() (net.Conn, error) { return nil, nil }
func (l *fakeListener) Close() error              { return nil }
func (l *fakeListener) Addr() net.Addr            { return nil }

type fakeCollector struct{}

func (c *fakeCollector) Describe(chan<- *prometheus.Desc) {}
func (c *fakeCollector) Collect(chan<- prometheus.Metric) {}

func mockUpHTTPServer(t *testing.T) (*mockHttpServer, func()) {
	httpServ := new(mockHttpServer)

	oldNewHTTPServer := newHTTPServer
	newHTTPServer = func(handler http.Handler) httpServer {
		return httpServ
	}

	deferFn := func() {
		newHTTPServer = oldNewHTTPServer
		httpServ.AssertExpectations(t)
	}

	return httpServ, deferFn
}

func TestDefaultServer_Start(t *testing.T) {
	testCases := map[string]struct {
		httpServeError error
	}{
		"http serve returns an error":        {httpServeError: errors.New("test error")},
		"http serve doesn't return an error": {httpServeError: nil},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			httpServ, deferFn := mockUpHTTPServer(t)
			defer deferFn()

			httpServ.On("Serve", mock.Anything).Return(testCase.httpServeError).Once()

			s := &defaultServer{}

			err := s.Start(new(fakeListener))
			assert.Equal(t, testCase.httpServeError, err)
		})
	}
}

func TestDefaultServer_RegisterPrometheusCollector(t *testing.T) {
	testCases := map[string]struct {
		registryError error
	}{
		"registry returns an error":        {registryError: errors.New("test error")},
		"registry doesn't return an error": {registryError: nil},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			registry := new(mockPrometheusRegistry)
			defer registry.AssertExpectations(t)

			registry.On("Register", mock.Anything).Return(testCase.registryError).Once()

			s := &defaultServer{prometheusRegistry: registry}

			err := s.RegisterPrometheusCollector(new(fakeCollector))
			assert.Equal(t, testCase.registryError, err)
		})
	}
}

func TestDefaultServer_RegisterPrometheusCollectors(t *testing.T) {
	testCases := map[string]struct {
		registerError error
	}{
		"register returns an error":        {registerError: errors.New("test error")},
		"register doesn't return an error": {registerError: nil},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			collector1 := new(fakeCollector)

			collectors := CollectorsMap{
				"collector 1": collector1,
			}

			registry := new(mockPrometheusRegistry)
			defer registry.AssertExpectations(t)

			registry.On("Register", collector1).Return(testCase.registerError).Once()

			s := &defaultServer{prometheusRegistry: registry}
			err := s.RegisterPrometheusCollectors(collectors)

			if testCase.registerError != nil {
				assert.EqualError(t, err, fmt.Sprintf("error while registering \"collector 1\" Prometheus collector: test error"))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultServer_RegisterDebugEndpoint(t *testing.T) {
	paths := map[string]string{
		"some/path":   "/debug/some/path",
		"some/path/":  "/debug/some/path/",
		"/some/path":  "/debug/some/path",
		"/some/path/": "/debug/some/path/",
	}

	for path, expectedPath := range paths {
		t.Run(path, func(t *testing.T) {
			mux := new(mockServeMux)
			defer mux.AssertExpectations(t)

			mux.On("HandleFunc", expectedPath, mock.Anything).Once()

			s := &defaultServer{mux: mux}
			s.RegisterDebugEndpoint(path, func(w http.ResponseWriter, r *http.Request) {})
		})
	}
}

func TestDefaultServer_RegisterDebugEndpoints(t *testing.T) {
	endpoints := EndpointsMap{
		"/some/path/1": func(w http.ResponseWriter, r *http.Request) {},
		"/some/path/2": func(w http.ResponseWriter, r *http.Request) {},
	}

	mux := new(mockServeMux)
	defer mux.AssertExpectations(t)

	mux.On("HandleFunc", "/debug/some/path/1", mock.Anything).Once()
	mux.On("HandleFunc", "/debug/some/path/2", mock.Anything).Once()

	s := &defaultServer{mux: mux}
	s.RegisterDebugEndpoints(endpoints)
}

func mockUpServeMux(t *testing.T) (*mockServeMux, *bool, func()) {
	mux := new(mockServeMux)

	oldNewServeMux := newServeMux
	deferFn := func() {
		newServeMux = oldNewServeMux
		mux.AssertExpectations(t)
	}

	newServeMuxCalled := false
	newServeMux = func() serveMux {
		newServeMuxCalled = true

		return mux
	}

	return mux, &newServeMuxCalled, deferFn
}

func mockUpPrometheusRegistry(t *testing.T) (*mockPrometheusRegistry, *bool, func()) {
	registry := new(mockPrometheusRegistry)

	oldNewPrometheusRegistry := newPrometheusRegistry
	deferFn := func() {
		newPrometheusRegistry = oldNewPrometheusRegistry
		registry.AssertExpectations(t)
	}

	newPrometheusRegistryCalled := false
	newPrometheusRegistry = func() prometheusRegistry {
		newPrometheusRegistryCalled = true

		return registry
	}

	return registry, &newPrometheusRegistryCalled, deferFn
}

func TestDefaultServerInitialization(t *testing.T) {
	testCases := map[string]struct {
		prometheusRegisterError error
	}{
		"prometheus register returns an error":        {prometheusRegisterError: errors.New("test error")},
		"prometheus register doesn't return an error": {prometheusRegisterError: nil},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			mux, newServeMuxCalled, muxDeferFn := mockUpServeMux(t)
			registry, newPrometheusRegistryCalled, registryDeferFn := mockUpPrometheusRegistry(t)

			defer muxDeferFn()
			defer registryDeferFn()

			mux.On("Handle", "/metrics", mock.Anything).Once()
			if testCase.prometheusRegisterError == nil {
				mux.On("HandleFunc", "/debug/pprof/", mock.Anything).Once()
				mux.On("HandleFunc", "/debug/pprof/cmdline", mock.Anything).Once()
				mux.On("HandleFunc", "/debug/pprof/profile", mock.Anything).Once()
				mux.On("HandleFunc", "/debug/pprof/symbol", mock.Anything).Once()
				mux.On("HandleFunc", "/debug/pprof/trace", mock.Anything).Once()

				registry.On("Register", mock.Anything).Return(nil).Twice()
			} else {
				registry.On("Register", mock.Anything).Return(testCase.prometheusRegisterError).Once()
			}

			server, err := newDefaultServer()

			assert.True(t, *newServeMuxCalled, "newServeMux() needs to be called")
			assert.True(t, *newPrometheusRegistryCalled, "newPrometheusRegistry() needs to be called")

			if testCase.prometheusRegisterError == nil {
				assert.NotNil(t, server)
				assert.NoError(t, err)
			} else {
				assert.Nil(t, server)
				assert.Error(t, err)
			}
		})
	}
}
