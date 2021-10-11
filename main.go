package main

import (
	"errors"
	"flag"
	"github.com/crazycs520/continuous-profile/discovery"
	"os"

	"github.com/crazycs520/continuous-profile/config"
	"github.com/crazycs520/continuous-profile/scrape"
	"github.com/crazycs520/continuous-profile/store"
	"github.com/crazycs520/continuous-profile/util/logutil"
	"github.com/crazycs520/continuous-profile/util/signal"
	"github.com/crazycs520/continuous-profile/web"
	"github.com/pingcap/log"
)

const (
	nmHost    = "host"
	nmPort    = "port"
	nmConfig  = "config"
	nmLogFile = "log-file"
)

var (
	host       = flag.String(nmHost, config.DefHost, "http server host")
	port       = flag.Uint(nmPort, config.DefPort, "http server port")
	configPath = flag.String(nmConfig, "", "config file path")
	logFile    = flag.String(nmLogFile, "", "log file name")
)

func main() {
	flag.Parse()

	err := config.Initialize(*configPath, overrideConfig)
	mustBeNil(err)

	setupLog()

	cfg := config.GetGlobalConfig()
	storage, err := store.NewProfileStorage(cfg.StorePath)
	mustBeNil(err)

	if cfg.PDAddr == "" {
		mustBeNil(errors.New("need specify PD address"))
	}
	discoverer, err := discovery.NewTopologyDiscoverer(cfg.PDAddr, cfg.Security.GetTLSConfig())
	mustBeNil(err)

	manager := scrape.NewManager(storage, discoverer.Subscribe())
	manager.Start()
	discoverer.Start()

	server := web.CreateHTTPServer(cfg.Host, cfg.Port, storage)
	err = server.StartServer()
	mustBeNil(err)

	exited := make(chan struct{})
	signal.SetupSignalHandler(func(graceful bool) {
		manager.Close()
		server.Close()
		close(exited)
	})
	<-exited
}

func setupLog() {
	cfg := config.GetGlobalConfig()
	err := logutil.InitLogger(cfg.Log.ToLogConfig())
	mustBeNil(err)
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
	if actualFlags[nmLogFile] {
		cfg.Log.Filename = *logFile
	}
}

func mustBeNil(err error) {
	if err == nil {
		return
	}
	log.Error(err.Error())
	os.Exit(-1)
}
