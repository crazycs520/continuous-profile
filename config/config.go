package config

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"sync/atomic"

	"github.com/crazycs520/continuous-profile/util/logutil"
	"github.com/pingcap/log"
	commonconfig "github.com/prometheus/common/config"
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
	err := cfg.load(configFile)
	if err != nil {
		return err
	}
	if overrideConfig != nil {
		overrideConfig(cfg)
	}
	StoreGlobalConfig(cfg)
	return nil
}

func (c *Config) load(configFile string) error {
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
	return err
}

func (c *Config) GetHTTPScheme() string {
	if c.Security.GetTLSConfig() != nil {
		return "https"
	}
	return "http"
}

func (c Config) String() string {
	b, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Sprintf("<error creating config string: %s>", err)
	}
	return string(b)
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

func (s *Security) GetHTTPClientConfig() commonconfig.HTTPClientConfig {
	return commonconfig.HTTPClientConfig{
		TLSConfig: commonconfig.TLSConfig{
			CAFile:   s.SSLCA,
			CertFile: s.SSLCert,
			KeyFile:  s.SSLKey,
		},
	}
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
