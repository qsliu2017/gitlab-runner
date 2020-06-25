package probe

import (
	context "context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func setupProbeServer(t *testing.T, fn func(host, port string)) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		switch r.Header.Get("X-Custom-Header") {
		case "fail":
			http.Error(w, "fail", http.StatusBadRequest)
			return

		case "redirect":
			http.Redirect(w, r, "http://nope.example.com:1234", http.StatusPermanentRedirect)
			return
		}

		if timeout := query.Get("timeout"); timeout != "" {
			sleep, err := time.ParseDuration(timeout)
			assert.NoError(t, err)

			time.Sleep(sleep)
		}

		status, _ := strconv.ParseInt(query.Get("status"), 10, 64)
		if status == 0 {
			return
		}
		w.WriteHeader(int(status))
	}))

	t.Cleanup(func() {
		ts.Close()
	})

	uri, _ := url.Parse(ts.URL)
	host, port, _ := net.SplitHostPort(uri.Host)

	fn(host, port)
}

func TestProbes(t *testing.T) {
	setupProbeServer(t, func(host, port string) {
		tests := map[string]struct {
			prober        Prober
			config        Config
			expectedDelay time.Duration
			failure       bool
		}{
			"tcp probe": {
				prober: &TCPProbe{Host: host, Port: port},
			},
			"tcp probe initial delay": {
				prober:        &TCPProbe{Host: host, Port: port},
				config:        Config{InitialDelay: 2 * time.Second},
				expectedDelay: 2 * time.Second,
			},
			"tcp probe timeout retries": {
				prober:        &TCPProbe{Host: "nope.example.com", Port: "9876"},
				config:        Config{Retries: 2, Timeout: 2 * time.Second},
				expectedDelay: 4 * time.Second,
				failure:       true,
			},
			"http get probe": {
				prober: &HTTPGetProbe{Host: host, Port: port},
			},
			"http get probe initial delay": {
				prober:        &HTTPGetProbe{Host: host, Port: port},
				config:        Config{InitialDelay: 2 * time.Second},
				expectedDelay: 2 * time.Second,
			},
			"http get probe timeout": {
				prober:        &HTTPGetProbe{Host: host, Port: port, Path: "?timeout=2s"},
				config:        Config{Timeout: 3 * time.Second},
				expectedDelay: 2 * time.Second,
			},
			"http get probe timeout retries": {
				prober:        &HTTPGetProbe{Host: host, Port: port, Path: "?timeout=3s"},
				config:        Config{Retries: 2, Timeout: 2 * time.Second},
				expectedDelay: 4 * time.Second,
				failure:       true,
			},
			"http get probe bad status": {
				prober:  &HTTPGetProbe{Host: host, Port: port, Path: "?status=400"},
				failure: true,
			},
			"http get probe header": {
				prober:  &HTTPGetProbe{Host: host, Port: port, Headers: []string{"X-Custom-Header: fail"}},
				failure: true,
			},
			"http get prone redirect": {
				prober:  &HTTPGetProbe{Host: host, Port: port, Headers: []string{"X-Custom-Header: redirect"}},
				failure: true,
			},
		}

		for tn := range tests {
			func(tn string) {
				t.Run(tn, func(t *testing.T) {
					t.Parallel()
					tc := tests[tn]

					expectedTime := time.Now().Add(tc.expectedDelay)

					success := Probe(context.Background(), logrus.StandardLogger(), tc.prober, tc.config)

					assert.NotEqual(t, tc.failure, success)
					assert.WithinDuration(t, expectedTime, time.Now(), 500*time.Millisecond)
				})
			}(tn)
		}
	})
}
