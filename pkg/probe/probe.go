package probe

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

type Node struct {
	ID    string
	Raw   string
	Proto string
	Host  string
	Port  int
	Path  string
	TLS   bool
	SNI   string
}

type Result struct {
	Success bool
	Latency time.Duration
	Method  string
	Err     string
}

type Options struct {
	Timeout       time.Duration
	HTTPProbePath string
}

func tcpProbe(ctx context.Context, host string, port int, timeout time.Duration) Result {
	start := time.Now()
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return Result{Success: false, Err: err.Error()}
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))
	_ = conn.Close()
	return Result{Success: true, Latency: time.Since(start), Method: "tcp"}
}

func tlsProbe(ctx context.Context, host string, port int, sni string, timeout time.Duration) Result {
	start := time.Now()
	d := net.Dialer{Timeout: timeout}
	conn, err := tls.DialWithDialer(&d, "tcp", fmt.Sprintf("%s:%d", host, port),
		&tls.Config{ServerName: sni, InsecureSkipVerify: true})
	if err != nil {
		return Result{Success: false, Err: err.Error()}
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))
	_ = conn.Close()
	return Result{Success: true, Latency: time.Since(start), Method: "tls"}
}

func httpProbe(ctx context.Context, host string, port int, path string, tlsOn bool, hostHeader string, timeout time.Duration) Result {
	start := time.Now()
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
			MaxIdleConnsPerHost: 10,
		},
	}
	scheme := "http"
	if tlsOn || port == 443 {
		scheme = "https"
	}
	if path == "" {
		path = "/"
	}
	url := fmt.Sprintf("%s://%s:%d%s", scheme, host, port, path)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	if hostHeader != "" {
		req.Host = hostHeader
	}
	resp, err := client.Do(req)
	if err != nil {
		return Result{Success: false, Err: err.Error()}
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 400 {
		return Result{Success: false, Err: fmt.Sprintf("http_status_%d", resp.StatusCode)}
	}
	return Result{Success: true, Latency: time.Since(start), Method: "http"}
}

type Origin interface {
	Name() string
	ProbeNode(ctx context.Context, n Node, opt Options) Result
}

type LocalOrigin struct{}

func (LocalOrigin) Name() string { return "local" }

func (LocalOrigin) ProbeNode(ctx context.Context, n Node, opt Options) Result {
	if n.Host == "" || n.Port == 0 {
		return Result{Success: true, Latency: 0, Method: "untested"}
	}
	timeout := opt.Timeout
	// prefer http if path/ws given
	if n.Path != "" {
		r := httpProbe(ctx, n.Host, n.Port, n.Path, n.TLS, n.SNI, timeout)
		if r.Success {
			return r
		}
	}
	// TLS if needed
	if n.TLS {
		r := tlsProbe(ctx, n.Host, n.Port, n.SNI, timeout)
		if r.Success {
			return r
		}
	}
	// fallback tcp
	r := tcpProbe(ctx, n.Host, n.Port, timeout)
	if r.Success {
		return r
	}
	return r
}

type AgentOrigin struct {
	name  string
	URL   string
	Token string
	HTTP  *http.Client
}

func (a AgentOrigin) Name() string { return a.name }

// Agent’s simple /probe endpoint accepts json node and returns {success, latency_ms, method, err}
func (a AgentOrigin) ProbeNode(ctx context.Context, n Node, opt Options) Result {
	if a.HTTP == nil {
		a.HTTP = &http.Client{Timeout: opt.Timeout + 2*time.Second}
	}
	reqBody := map[string]any{
		"id": n.ID, "raw": n.Raw, "proto": n.Proto, "host": n.Host, "port": n.Port,
		"path": n.Path, "tls": n.TLS, "sni": n.SNI, "timeout_ms": opt.Timeout.Milliseconds(),
	}
	resp, err := doJSON(ctx, a.HTTP, a.URL+"/probe", a.Token, reqBody)
	if err != nil {
		return Result{Success: false, Err: err.Error()}
	}
	ok, _ := resp["success"].(bool)
	method, _ := resp["method"].(string)
	errStr, _ := resp["err"].(string)
	lat := time.Duration(0)
	if v, ok := resp["latency_ms"].(float64); ok {
		lat = time.Duration(int64(v)) * time.Millisecond
	}
	return Result{Success: ok, Latency: lat, Method: "agent:" + method, Err: errStr}
}

func doJSON(ctx context.Context, c *http.Client, url string, token string, payload map[string]any) (map[string]any, error) {
	j, _ := jsonMarshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytesReader(j))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, errors.New("agent_http_" + resp.Status)
	}
	var out map[string]any
	if err := jsonNewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// lightweight JSON helpers without adding deps
type rd struct{ b []byte }
func bytesReader(b []byte) *rd { return &rd{b} }
func (r *rd) Read(p []byte) (int, error) { if len(r.b)==0 { return 0, ioEOF{} }; n:=copy(p, r.b); r.b=r.b[n:]; return n, nil }
func (r *rd) Close() error { return nil }

type ioEOF struct{}
func (ioEOF) Error() string { return "EOF" }

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }
func jsonNewDecoder(r *rd) *json.Decoder { return json.NewDecoder(r) }

// we need imports for helper
// (kept here for simplicity)
import (
	"encoding/json"
	"io"
)