package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	TotalProbes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "v2mgr_probes_total", Help: "Total probes",
	}, []string{"result"})
	AvgLatency = prometheus.NewSummary(prometheus.SummaryOpts{
		Name: "v2mgr_latency_seconds", Help: "Probe latency",
	})
	Quarantines = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "v2mgr_quarantine_total", Help: "Total quarantines",
	})
	Deletions = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "v2mgr_deletions_total", Help: "Total deletions",
	})
)

func MustRegister() {
	prometheus.MustRegister(TotalProbes, AvgLatency, Quarantines, Deletions)
}