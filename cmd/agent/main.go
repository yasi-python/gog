package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/yasi-python/go/pkg/logger"
	"github.com/yasi-python/go/pkg/probe"
)

func main() {
	log := logger.New("info")
	mux := http.NewServeMux()
	mux.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {
		var req struct{
			ID string `json:"id"`
			Raw string `json:"raw"`
			Proto string `json:"proto"`
			Host string `json:"host"`
			Port int    `json:"port"`
			Path string `json:"path"`
			TLS  bool   `json:"tls"`
			SNI  string `json:"sni"`
			TimeoutMS int64 `json:"timeout_ms"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad_json", 400); return
		}
		ctx := r.Context()
		res := probe.LocalOrigin{}.ProbeNode(ctx, probe.Node{
			ID: req.ID, Raw: req.Raw, Proto: req.Proto, Host: req.Host, Port: req.Port, Path: req.Path, TLS: req.TLS, SNI: req.SNI,
		}, probe.Options{Timeout: time.Duration(req.TimeoutMS)*time.Millisecond})
		out := map[string]any{"success": res.Success, "latency_ms": res.Latency.Milliseconds(), "method": res.Method, "err": res.Err}
		_ = json.NewEncoder(w).Encode(out)
	})
	addr := ":8081"
	log.Info("agent_listen", "addr", addr)
	_ = http.ListenAndServe(addr, mux)
}