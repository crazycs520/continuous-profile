package main

import (
	"flag"
	"github.com/crazycs520/continuous-profile/config"
)

const (
	nmHost   = "host"
	nmPort   = "port"
	nmConfig = "config"
)

var (
	host       = flag.String(nmHost, "0.0.0.0", "http server host")
	port       = flag.Uint(nmPort, 4000, "http server port")
	configPath = flag.String(nmConfig, "", "config file path")
)

func main() {
	flag.Parse()

	config.Initialize(*configPath, overrideConfig)
}

func overrideConfig(cfg *config.Config) {
	actualFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		actualFlags[f.Name] = true
	})

	if actualFlags[nmConfig] {
		cfg.ConfigPath = *configPath
	}
	if actualFlags[nmHost] {
		cfg.Host = *host
	}
	if actualFlags[nmPort] {
		cfg.Port = *port
	}
}
