package main

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/yasi-python/go/internal/subscription"
	"github.com/yasi-python/go/pkg/api"
	"github.com/yasi-python/go/pkg/config"
	"github.com/yasi-python/go/pkg/decision"
	"github.com/yasi-python/go/pkg/logger"
	"github.com/yasi-python/go/pkg/metrics"
	"github.com/yasi-python/go/pkg/probe"
	"github.com/yasi-python/go/pkg/storage"
)

type Manager struct {
	cfg    *config.Config
	log    *logger.Logger
	db     *storage.DB
	origins []probe.Origin
	snapDir string

	// state
	totalDeletionsToday int
	dayStart time.Time
}

func NewManager(cfg *config.Config, log *logger.Logger, db *storage.DB) *Manager {
	origins := []probe.Origin{}
	for _, o := range cfg.Origins {
		if o.Type == "local" {
			origins = append(origins, probe.LocalOrigin{})
				} else if o.Type == "agent" && o.URL != "" {
					origins = append(origins, probe.AgentOrigin{
						Label: o.Name, URL: o.URL, Token: o.Token,
					})
				}
	}
	return &Manager{
		cfg: cfg, log: log, db: db, origins: origins, snapDir: cfg.Service.SnapshotsDir,
		dayStart: midnightUTC(time.Now()),
	}
}

func midnightUTC(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0,0,0,0, time.UTC)
}

func (m *Manager) maybeResetDailyCounters(now time.Time) {
	if now.UTC().Day() != m.dayStart.Day() {
		m.dayStart = midnightUTC(now)
		m.totalDeletionsToday = 0
	}
}

func (m *Manager) ListConfigs() any {
	cs, _ := m.db.ListConfigs()
	return cs
}

func (m *Manager) Reprobe(id string) error {
	c, err := m.db.GetConfig(id)
	if err != nil { return err }
	return m.probeOnceAndDecide(*c)
}

func (m *Manager) Quarantine(id string) error {
	c, err := m.db.GetConfig(id)
	if err != nil { return err }
	c.Quarantine = true
	return m.db.PutConfig(*c)
}

func (m *Manager) Delete(id string) error {
	if m.cfg.Service.DryRun || !m.cfg.Security.AllowDelete {
		return fmt.Errorf("delete_disabled_dryrun_or_security")
	}
	m.maybeResetDailyCounters(time.Now())
	if m.totalDeletionsToday >= m.cfg.Service.MaxDeletionsPerDay {
		return fmt.Errorf("deletions_throttled")
	}
	c, err := m.db.GetConfig(id)
	if err != nil { return err }
	_, _ = m.db.SnapshotConfig(*c, m.snapDir)
	c.Deleted = true
	if err := m.db.PutConfig(*c); err != nil { return err }
	m.totalDeletionsToday++
	return nil
}

func (m *Manager) Rollback(id string) error {
	// naive: mark as not deleted. (restoring full attributes is done by reading snapshot manually; kept simple)
	c, err := m.db.GetConfig(id)
	if err != nil { return err }
	c.Deleted = false
	return m.db.PutConfig(*c)
}

func parseMinimal(raw string) (proto, host string, port int, path string, tlsOn bool, sni string) {
	l := strings.TrimSpace(strings.ToLower(raw))
	_ = l
	// very light parse: try find host:port first
	host, port = findHostPort(raw)
	// protocol
	if strings.HasPrefix(l, "vmess://") { proto = "vmess" }
	if strings.HasPrefix(l, "vless://") { proto = "vless" }
	if strings.HasPrefix(l, "trojan://") { proto = "trojan" }
	if strings.HasPrefix(l, "ss://") { proto = "ss" }
	if strings.HasPrefix(l, "socks5://") { proto = "socks5" }
	// path/sni heuristics
	if i := strings.Index(raw, "path="); i >= 0 {
		path = "/" // minimal
	}
	if strings.Contains(raw, "security=tls") || strings.Contains(raw, "tls=") {
		tlsOn = true
	}
	return
}

func findHostPort(raw string) (string, int) {
	// crude host:port detector
	for _, seg := range strings.Fields(strings.ReplaceAll(raw, "/", " ")) {
		if i := strings.LastIndex(seg, ":"); i > 0 && i < len(seg)-1 {
			h := seg[:i]
			var p int
			_, err := fmt.Sscanf(seg[i+1:], "%d", &p)
			if err == nil && p > 0 && p < 65536 {
				return trimHost(h), p
			}
		}
	}
	return "", 0
}

func trimHost(h string) string {
	h = strings.Trim(h, "[]")
	h = strings.TrimSuffix(h, ",")
	return h
}

func idFor(raw string) string {
	h := sha1.Sum([]byte(raw))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func (m *Manager) mergeAndStore(ctx context.Context) ([]storage.ConfigRecord, error) {
	f := subscription.HTTPFetcher{}
	all := []string{}
	for _, u := range m.cfg.Subscriptions.Sources {
		txt, err := f.Fetch(ctx, u)
		if err != nil {
			m.log.Warn("fetch_failed", "url", u, "err", err.Error())
			continue
		}
		nodes := subscription.ExtractNodes(txt)
		if m.cfg.Subscriptions.PerSourceLimit > 0 && len(nodes) > m.cfg.Subscriptions.PerSourceLimit {
			nodes = nodes[:m.cfg.Subscriptions.PerSourceLimit]
		}
		all = append(all, nodes...)
	}
	// dedupe
	seen := map[string]bool{}
	candidates := []string{}
	for _, n := range all {
		if !seen[n] {
			seen[n] = true
			candidates = append(candidates, n)
		}
		if m.cfg.Subscriptions.MergedLimit > 0 && len(candidates) >= m.cfg.Subscriptions.MergedLimit {
			break
		}
	}
	out := []storage.ConfigRecord{}
	for _, raw := range candidates {
		id := idFor(raw)
		proto, host, port, path, tlsOn, sni := parseMinimal(raw)
		cr := storage.ConfigRecord{ID: id, Raw: raw, Proto: proto, Host: host, Port: port}
		_ = path; _ = tlsOn; _ = sni // stored minimal
		if err := m.db.PutConfig(cr); err == nil {
			out = append(out, cr)
		}
	}
	return out, nil
}

func (m *Manager) probeOnceAndDecide(c storage.ConfigRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(m.cfg.Probe.TimeoutMS)*time.Millisecond)
	defer cancel()
	opt := probe.Options{Timeout: time.Duration(m.cfg.Probe.TimeoutMS) * time.Millisecond}
	// run across origins; we require consensus: all must succeed to record success
	successAll := true
	latAgg := time.Duration(0)
	tried := 0
	for _, o := range m.origins {
		tried++
		n := probe.Node{ID: c.ID, Raw: c.Raw, Proto: c.Proto, Host: c.Host, Port: c.Port}
		res := o.ProbeNode(ctx, n, opt)
		if res.Success {
			metrics.TotalProbes.WithLabelValues("success").Inc()
			metrics.AvgLatency.Observe(res.Latency.Seconds())
			latAgg += res.Latency
		} else {
			metrics.TotalProbes.WithLabelValues("failure").Inc()
			successAll = false
		}
	}
	statsRec, err := m.db.UpdateStatsForProbe(c.ID, successAll && tried>0)
	if err != nil { return err }
	// Decision
	dec := decision.Evaluate(decision.DecisionInput{
		Stats: decision.ConfigStats{
			ID: c.ID, Attempts: statsRec.Attempts, Successes: statsRec.Successes,
			Failures: statsRec.Failures, ConsecutiveFailures: statsRec.ConsecutiveFailures,
			LastFailureUnix: statsRec.LastFailureUnix, LastSuccessUnix: statsRec.LastSuccessUnix,
		},
		Z: m.cfg.Decision.DecisionConfidenceZ,
		MinAttempts: m.cfg.Decision.MinAttemptsForDecision,
		DeleteLB: m.cfg.Decision.DeleteLowerBoundThreshold,
		ConsecFailToQ: m.cfg.Decision.QuarantineConsecutiveFailures,
		Now: time.Now(),
	})
	switch dec.Action {
	case decision.ActionQuarantine:
		c.Quarantine = true
		_ = m.db.PutConfig(c)
		metrics.Quarantines.Inc()
		m.log.Warn("quarantine", "id", c.ID, "reason", dec.Reason)
	case decision.ActionDelete:
		// safety: dry-run + allow_delete + daily throttle
		if !m.cfg.Service.DryRun && m.cfg.Security.AllowDelete {
			if err := m.Delete(c.ID); err != nil {
				m.log.Error("delete_failed", "id", c.ID, "err", err.Error())
			} else {
				metrics.Deletions.Inc()
				m.log.Warn("deleted", "id", c.ID, "failure_lb", fmt.Sprintf("%.6f", dec.FailureLB))
			}
		} else {
			m.log.Warn("would_delete_dryrun_or_disabled", "id", c.ID, "failure_lb", fmt.Sprintf("%.6f", dec.FailureLB))
		}
	default:
		// keep
	}
	return nil
}

func (m *Manager) backgroundLoop(ctx context.Context) {
	tickerFetch := time.NewTicker(time.Duration(m.cfg.Subscriptions.FetchIntervalSeconds) * time.Second)
	tickerProbe := time.NewTicker(time.Duration(m.cfg.Service.ReprobeScheduleSeconds) * time.Second)
	defer tickerFetch.Stop()
	defer tickerProbe.Stop()

	// initial fetch
	_, _ = m.mergeAndStore(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-tickerFetch.C:
			_, _ = m.mergeAndStore(ctx)
		case <-tickerProbe.C:
			cs, _ := m.db.ListConfigs()
			// bounded concurrency
			sem := make(chan struct{}, m.cfg.Service.Concurrency)
			for _, c := range cs {
				if c.Deleted {
					continue
				}
				sem <- struct{}{}
				go func(cc storage.ConfigRecord) {
					defer func(){ <-sem }()
					_ = m.probeOnceAndDecide(cc)
				}(c)
			}
			// drain
			for i := 0; i < cap(sem); i++ {
				sem <- struct{}{}
			}
		}
	}
}

func main() {
	cfgPath := "config.yaml"
	if len(os.Args) > 1 && os.Args[1] != "" {
		cfgPath = os.Args[1]
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Println("config_load_error:", err.Error()); os.Exit(2)
	}
	log := logger.New(cfg.Service.LogLevel)
	metrics.MustRegister()
	if err := os.MkdirAll(filepath.Dir(cfg.Service.DataDir+"/db.bolt"), 0o755); err != nil {
		log.Error("mkdir_data", "err", err.Error()); os.Exit(2)
	}
	db, err := storage.Open(filepath.Join(cfg.Service.DataDir, "db.bolt"))
	if err != nil {
		log.Error("db_open", "err", err.Error()); os.Exit(2)
	}
	defer db.Close()

	mgr := NewManager(cfg, log, db)

	// API server
	apiSrv := api.New(mgr, cfg.Service.MetricsPath, cfg.Service.HealthzPath)
	go func(){
		if err := apiSrv.Start(cfg.Service.HTTPListen); err != nil {
			log.Error("api_start", "err", err.Error())
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	log.Info("manager_start", "listen", cfg.Service.HTTPListen, "dry_run", cfg.Service.DryRun)
	mgr.backgroundLoop(ctx)
}