package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type ServiceCfg struct {
	HTTPListen                  string `yaml:"http_listen"`
	MetricsPath                 string `yaml:"metrics_path"`
	HealthzPath                 string `yaml:"healthz_path"`
	DryRun                      bool   `yaml:"dry_run"`
	LogLevel                    string `yaml:"log_level"`
	DataDir                     string `yaml:"data_dir"`
	SnapshotsDir                string `yaml:"snapshots_dir"`
	SnapshotRetentionDays       int    `yaml:"snapshot_retention_days"`
	MaxDeletionsPerDay          int    `yaml:"max_deletions_per_day"`
	Concurrency                 int    `yaml:"concurrency"`
	RateLimitPerTargetPerMinute int    `yaml:"rate_limit_per_target_per_minute"`
	ReprobeScheduleSeconds      int    `yaml:"reprobe_schedule_seconds"`
}

type SubscriptionsCfg struct {
	Sources              []string `yaml:"sources"`
	FetchIntervalSeconds int      `yaml:"fetch_interval_seconds"`
	PerSourceLimit       int      `yaml:"per_source_limit"`
	MergedLimit          int      `yaml:"merged_limit"`
	Outputs              struct {
		PlainPath  string `yaml:"plain_path"`
		Base64Path string `yaml:"base64_path"`
	} `yaml:"outputs"`
}

type ProbeCfg struct {
	TimeoutMS               int      `yaml:"timeout_ms"`
	Retries                 int      `yaml:"retries"`
	BackoffInitialMS        int      `yaml:"backoff_initial_ms"`
	BackoffMaxMS            int      `yaml:"backoff_max_ms"`
	HTTPProbePaths          []string `yaml:"http_probe_paths"`
	PreferHTTPIfWSOrPath    bool     `yaml:"prefer_http_if_ws_or_path"`
}

type Origin struct {
	Name   string `yaml:"name"`
	Type   string `yaml:"type"` // local|agent
	URL    string `yaml:"url"`
	Token  string `yaml:"token"`
	Weight int    `yaml:"weight"`
}

type DecisionCfg struct {
	MinAttemptsForDecision        int      `yaml:"min_attempts_for_decision"`
	DecisionConfidenceZ           float64  `yaml:"decision_confidence_z"`
	QuarantineConsecutiveFailures int      `yaml:"quarantine_consecutive_failures"`
	QuarantineRechecks            []string `yaml:"quarantine_rechecks"`
	DeleteLowerBoundThreshold     float64  `yaml:"delete_lower_bound_threshold"`
}

type SecurityCfg struct {
	AllowDelete bool     `yaml:"allow_delete"`
	BlacklistIPs []string `yaml:"blacklist_ips"`
}

type APICfg struct {
	RateLimitPerMinute int `yaml:"rate_limit_per_minute"`
}

type Config struct {
	Service       ServiceCfg       `yaml:"service"`
	Subscriptions SubscriptionsCfg `yaml:"subscriptions"`
	Probe         ProbeCfg         `yaml:"probe"`
	Origins       []Origin         `yaml:"origins"`
	Decision      DecisionCfg      `yaml:"decision"`
	Security      SecurityCfg      `yaml:"security"`
	API           APICfg           `yaml:"api"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	if c.Service.Concurrency <= 0 {
		c.Service.Concurrency = 100
	}
	return &c, nil
}

func (c *Config) QuarantineRechecksDurations() []time.Duration {
	var out []time.Duration
	for _, s := range c.Decision.QuarantineRechecks {
		if d, err := time.ParseDuration(s); err == nil {
			out = append(out, d)
		}
	}
	return out
}