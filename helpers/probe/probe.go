package probe

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Logger interface {
	Warningln(args ...interface{})
}

// Prober is an interface used for implementing a specific probe.
type Prober interface {
	Probe(ctx context.Context, timeout time.Duration) error
	String() string
}

type Config struct {
	Retries      int
	InitialDelay time.Duration
	Period       time.Duration
	Timeout      time.Duration
}

// Probe executes the probe provided, retrying if there's a failure after the specified
// intervals.
func Probe(ctx context.Context, logger Logger, prober Prober, config Config) bool {
	time.Sleep(config.InitialDelay)

	if config.Retries < 1 {
		config.Retries = 1
	}

	for retry := 0; retry < config.Retries; retry++ {
		if retry > 0 {
			time.Sleep(config.Period)
		}

		err := prober.Probe(ctx, config.Timeout)
		if err == nil {
			return true
		}

		logger.Warningln(fmt.Sprintf("Probe %q (%d/%d):", prober, retry+1, config.Retries), err)
	}

	return false
}

type TCPProbe struct {
	Host string
	Port string
}

func (p *TCPProbe) String() string {
	return net.JoinHostPort(p.Host, p.Port)
}

func (p *TCPProbe) Probe(ctx context.Context, timeout time.Duration) error {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(p.Host, p.Port))
	if err != nil {
		return err
	}
	defer conn.Close()

	return nil
}

type HTTPGetProbe struct {
	Host    string
	Port    string
	Scheme  string
	Path    string
	Headers []string
}

func (p *HTTPGetProbe) String() string {
	return fmt.Sprintf("url=%v, headers=%v", &url.URL{
		Host:   net.JoinHostPort(p.Host, p.Port),
		Scheme: p.Scheme,
		Path:   p.Path,
	}, p.Headers)
}

func (p *HTTPGetProbe) Probe(ctx context.Context, timeout time.Duration) error {
	if p.Scheme == "" {
		p.Scheme = "http"
	}

	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return errors.New("redirections are not allowed")
		},
	}

	u, err := url.Parse(p.Path)
	if err != nil {
		u = &url.URL{
			Path: p.Path,
		}
	}

	u.Scheme = p.Scheme
	u.Host = net.JoinHostPort(p.Host, p.Port)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}

	for _, hdr := range p.Headers {
		kv := strings.SplitN(hdr, ":", 2)
		if len(kv) < 2 {
			continue
		}
		req.Header.Add(kv[0], kv[1])
	}
	if req.Header.Get("Host") != "" {
		req.Host = req.Header.Get("Host")
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(io.LimitReader(resp.Body, 10*1024))
	if err != nil {
		return err
	}

	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusBadRequest {
		return nil
	}
	return fmt.Errorf("HTTP probe failed, status code: %d, body: %v", resp.StatusCode, string(body))
}
