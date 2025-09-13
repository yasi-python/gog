package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Manager interface {
	ListConfigs() any
	Reprobe(id string) error
	Quarantine(id string) error
	Delete(id string) error
	Rollback(id string) error
}

type Server struct {
	Mgr          Manager
	MetricsPath  string
	HealthzPath  string
	reqInFlight  atomic.Int64
}

func New(mgr Manager, metricsPath, healthzPath string) *Server {
	return &Server{Mgr: mgr, MetricsPath: metricsPath, HealthzPath: healthzPath}
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(s.HealthzPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200); _, _ = w.Write([]byte("ok"))
	})
	mux.Handle(s.MetricsPath, promhttp.Handler())

	mux.HandleFunc("/api/v1/configs", s.wrap(func(w http.ResponseWriter, r *http.Request) {
		sendJSON(w, 200, s.Mgr.ListConfigs())
	}))
	mux.HandleFunc("/api/v1/reprobe", s.wrap(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" { sendJSON(w, 400, errMsg("missing id")); return }
		if err := s.Mgr.Reprobe(id); err != nil { sendJSON(w, 500, errMsg(err.Error())); return }
		sendJSON(w, 200, okMsg("scheduled"))
	}))
	mux.HandleFunc("/api/v1/quarantine", s.wrap(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" { sendJSON(w, 400, errMsg("missing id")); return }
		if err := s.Mgr.Quarantine(id); err != nil { sendJSON(w, 500, errMsg(err.Error())); return }
		sendJSON(w, 200, okMsg("quarantined"))
	}))
	mux.HandleFunc("/api/v1/delete", s.wrap(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" { sendJSON(w, 400, errMsg("missing id")); return }
		if err := s.Mgr.Delete(id); err != nil { sendJSON(w, 500, errMsg(err.Error())); return }
		sendJSON(w, 200, okMsg("deleted"))
	}))
	mux.HandleFunc("/api/v1/rollback", s.wrap(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" { sendJSON(w, 400, errMsg("missing id")); return }
		if err := s.Mgr.Rollback(id); err != nil { sendJSON(w, 500, errMsg(err.Error())); return }
		sendJSON(w, 200, okMsg("rolled_back"))
	}))
	return mux
}

func (s *Server) wrap(h func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.reqInFlight.Add(1)
		defer s.reqInFlight.Add(-1)
		h(w,r)
	}
}

func (s *Server) Start(addr string) error {
	return http.ListenAndServe(addr, s.routes())
}

func sendJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type","application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func okMsg(m string) map[string]any { return map[string]any{"ok": true, "message": m} }
func errMsg(m string) map[string]any { return map[string]any{"ok": false, "error": m} }

var _ = fmt.Sprintf