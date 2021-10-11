package config

import "time"

// ScrapeConfig configures a scraping unit for conprof.
type ScrapeConfig struct {
	ComponentName string `yaml:"component_name,omitempty"`
	// How frequently to scrape the targets of this scrape config.
	ScrapeInterval time.Duration `yaml:"scrape_interval,omitempty"`
	// The timeout for scraping targets of this config.
	ScrapeTimeout time.Duration `yaml:"scrape_timeout,omitempty"`

	ProfilingConfig *ProfilingConfig `yaml:"profiling_config,omitempty"`
	Targets         []string         `yaml:"targets"`
}

type ProfilingConfig struct {
	PprofConfig PprofConfig `yaml:"pprof_config,omitempty"`
}

type PprofConfig map[string]*PprofProfilingConfig

type PprofProfilingConfig struct {
	Enabled *bool             `yaml:"enabled,omitempty"`
	Path    string            `yaml:"path,omitempty"`
	Seconds int               `yaml:"seconds"`
	Header  map[string]string `yaml:"header,omitempty"`
	Params  map[string]string `yaml:"params,omitempty"`
}

