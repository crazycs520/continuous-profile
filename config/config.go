package config

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/crazycs520/continuous-profile/util/logutil"
	"github.com/pingcap/log"
	commonconfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"go.etcd.io/etcd/pkg/transport"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

const (
	DefHost                          = "0.0.0.0"
	DefPort                          = 10092
	defStorePath                     = "data"
	DefProfilingIntervalSeconds      = 10
	DefProfileSeconds                = 5
	DefProfilingTimeoutSeconds       = 120
	DefProfilingDataRetentionSeconds = 3 * 24 * 60 * 60 // 3 days
)

type Config struct {
	Host              string                  `yaml:"host" json:"host"`
	Port              uint                    `yaml:"port" json:"port"`
	StorePath         string                  `yaml:"store_path" json:"store_path"`
	ConfigPath        string                  `yaml:"config_path" json:"config_path"`
	PDAddr            string                  `yaml:"pd_address" json:"pd_address"`
	Log               Log                     `yaml:"log" json:"log"`
	ContinueProfiling ContinueProfilingConfig `yaml:"-" json:"-"`
	ScrapeConfigs     []*ScrapeConfig         `yaml:"scrape_configs,omitempty" json:"scrape_configs,omitempty"`
	Security          Security                `yaml:"security" json:"security"`
}

var defaultConfig = Config{
	Host:      DefHost,
	Port:      DefPort,
	StorePath: defStorePath,
	ContinueProfiling: ContinueProfilingConfig{
		Enable:               false,
		ProfileSeconds:       DefProfileSeconds,
		IntervalSeconds:      DefProfilingIntervalSeconds,
		TimeoutSeconds:       DefProfilingTimeoutSeconds,
		DataRetentionSeconds: DefProfilingDataRetentionSeconds,
	},
	Log: Log{
		Level:   "info",
		MaxSize: logutil.DefaultLogMaxSize,
	},
}

type ContinueProfilingConfig struct {
	Enable               bool
	ProfileSeconds       int
	IntervalSeconds      int
	TimeoutSeconds       int
	DataRetentionSeconds int
}

// ScrapeConfig configures a scraping unit for conprof.
type ScrapeConfig struct {
	ComponentName string `yaml:"component_name,omitempty"`
	// A set of query parameters with which the target is scraped.
	Params url.Values `yaml:"params,omitempty"`
	// How frequently to scrape the targets of this scrape config.
	ScrapeInterval model.Duration `yaml:"scrape_interval,omitempty"`
	// The timeout for scraping targets of this config.
	ScrapeTimeout model.Duration `yaml:"scrape_timeout,omitempty"`
	// The URL scheme with which to fetch metrics from targets.
	Scheme string `yaml:"scheme,omitempty"`

	ProfilingConfig *ProfilingConfig `yaml:"profiling_config,omitempty"`

	Targets []string `yaml:"targets"`

	HTTPClientConfig commonconfig.HTTPClientConfig `yaml:",inline"`
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

var globalConf atomic.Value

func NewConfig() *Config {
	cfg := defaultConfig
	return &cfg
}

func GetGlobalConfig() *Config {
	return globalConf.Load().(*Config)
}

// StoreGlobalConfig stores a new config to the globalConf. It mostly uses in the test to avoid some data races.
func StoreGlobalConfig(config *Config) {
	globalConf.Store(config)
}

func Initialize(configFile string, overrideConfig func(*Config)) error {
	cfg := NewConfig()
	err := cfg.LoadAndCheck(configFile)
	if err != nil {
		return err
	}
	if overrideConfig != nil {
		overrideConfig(cfg)
	}
	StoreGlobalConfig(cfg)
	return nil
}

func (c *Config) LoadAndCheck(configFile string) error {
	if configFile == "" {
		return nil
	}
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return err
	}
	err = yaml.NewDecoder(bytes.NewReader(data)).Decode(c)
	if err != nil {
		return err
	}
	c.setDefaultFields()
	err = c.checkValid()
	return err
}

func (c *Config) GetHTTPScheme() string {
	if c.Security.GetTLSConfig() != nil {
		return "https"
	}
	return "http"
}

func (c *Config) checkValid() error {
	for _, scrape := range c.ScrapeConfigs {
		if scrape.ProfilingConfig == nil {
			continue
		}
		profileConf, ok := scrape.ProfilingConfig.PprofConfig["profile"]
		if !ok {
			continue
		}
		if profileConf.Seconds >= int(time.Duration(scrape.ScrapeTimeout).Seconds()) {
			return fmt.Errorf("job %v, profile.seconds(%v) should less than the scrapscrape_timeout(%v)",
				scrape.ComponentName, profileConf.Seconds, time.Duration(scrape.ScrapeTimeout).String())
		}
	}
	return nil
}

func (c *Config) setDefaultFields() {
	defScrape := defaultScrapeConfig()
	for i, scrape := range c.ScrapeConfigs {
		if time.Duration(scrape.ScrapeInterval).Nanoseconds() == 0 {
			scrape.ScrapeInterval = defScrape.ScrapeInterval
		}
		if time.Duration(scrape.ScrapeTimeout).Nanoseconds() == 0 {
			scrape.ScrapeTimeout = defScrape.ScrapeTimeout
		}
		if scrape.Scheme == "" {
			scrape.Scheme = defScrape.Scheme
		}

		if scrape.ProfilingConfig == nil {
			scrape.ProfilingConfig = defScrape.ProfilingConfig
		} else if len(scrape.ProfilingConfig.PprofConfig) == 0 {
			scrape.ProfilingConfig.PprofConfig = defScrape.ProfilingConfig.PprofConfig
		} else {
			for name, defPprofConf := range defScrape.ProfilingConfig.PprofConfig {
				conf, ok := scrape.ProfilingConfig.PprofConfig[name]
				if !ok {
					scrape.ProfilingConfig.PprofConfig[name] = &(*defPprofConf)
					continue
				}
				if conf.Enabled == nil {
					conf.Enabled = defPprofConf.Enabled
				}
				if conf.Seconds == 0 {
					conf.Seconds = defPprofConf.Seconds
				}
				if conf.Path == "" {
					conf.Path = defPprofConf.Path
				}
				scrape.ProfilingConfig.PprofConfig[name] = conf
			}
		}
		c.ScrapeConfigs[i] = scrape
	}
}

func defaultScrapeConfig() ScrapeConfig {
	return ScrapeConfig{
		ScrapeInterval: model.Duration(DefProfilingIntervalSeconds * time.Second),
		ScrapeTimeout:  model.Duration(DefProfilingTimeoutSeconds * time.Second),
		Scheme:         "http",
		ProfilingConfig: &ProfilingConfig{
			PprofConfig: PprofConfig{
				"allocs": &PprofProfilingConfig{
					Enabled: trueValue(),
					Path:    "/debug/pprof/allocs",
				},
				"block": &PprofProfilingConfig{
					Enabled: trueValue(),
					Path:    "/debug/pprof/block",
				},
				"goroutine": &PprofProfilingConfig{
					Enabled: trueValue(),
					Path:    "/debug/pprof/goroutine",
				},
				"heap": &PprofProfilingConfig{
					Enabled: trueValue(),
					Path:    "/debug/pprof/heap",
				},
				"mutex": &PprofProfilingConfig{
					Enabled: trueValue(),
					Path:    "/debug/pprof/mutex",
				},
				"profile": &PprofProfilingConfig{
					Enabled: trueValue(),
					Path:    "/debug/pprof/profile",
					Seconds: DefProfileSeconds,
				},
			},
		},
	}
}

func (c Config) String() string {
	b, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Sprintf("<error creating config string: %s>", err)
	}
	return string(b)
}

func trueValue() *bool {
	a := true
	return &a
}

// Log is the log section of config.
type Log struct {
	Level    string `yaml:"level" json:"level"`
	Filename string `yaml:"filename" json:"filename"`
	// Max size for a single file, in MB.
	MaxSize int `yaml:"max_size" json:"max_size"`
	// Max log keep days, default is never deleting.
	MaxDays int `yaml:"max_days" json:"max_days"`
	// Maximum number of old log files to retain.
	MaxBackups int `yaml:"max_backups" json:"max_backups"`
}

func (l *Log) ToLogConfig() *logutil.LogConfig {
	file := log.FileLogConfig{
		Filename:   l.Filename,
		MaxSize:    l.MaxSize,
		MaxDays:    l.MaxDays,
		MaxBackups: l.MaxBackups,
	}
	return logutil.NewLogConfig(l.Level, file)
}

type Security struct {
	SSLCA     string `yaml:"ssl-ca" json:"cluster-ssl-ca"`
	SSLCert   string `yaml:"ssl-cert" json:"cluster-ssl-cert"`
	SSLKey    string `yaml:"ssl-key" json:"cluster-ssl-key"`
	tlsConfig *tls.Config
}

func (s *Security) GetTLSConfig() *tls.Config {
	if s.tlsConfig != nil {
		return s.tlsConfig
	}
	if s.SSLCA == "" || s.SSLCert == "" || s.SSLKey == "" {
		return nil
	}
	s.tlsConfig = buildTLSConfig(s.SSLCA, s.SSLKey, s.SSLCert)
	return s.tlsConfig
}

func buildTLSConfig(caPath, keyPath, certPath string) *tls.Config {
	tlsInfo := transport.TLSInfo{
		TrustedCAFile: caPath,
		KeyFile:       keyPath,
		CertFile:      certPath,
	}
	tlsConfig, err := tlsInfo.ClientConfig()
	if err != nil {
		log.Fatal("Failed to load certificates", zap.Error(err))
	}
	return tlsConfig
}
