package main

import (
	"flag"
	"fmt"
	"github.com/crazycs520/continuous-profile/config"
	"github.com/crazycs520/continuous-profile/scrape"
	"github.com/crazycs520/continuous-profile/store"
	"github.com/crazycs520/continuous-profile/store/badger"
	"os"
)

const (
	nmHost   = "host"
	nmPort   = "port"
	nmConfig = "config"
)

var (
	host       = flag.String(nmHost, "0.0.0.0", "http server host")
	port       = flag.Uint(nmPort, 10101, "http server port")
	configPath = flag.String(nmConfig, "", "config file path")
)

func main() {
	flag.Parse()

	err := config.Initialize(*configPath, overrideConfig)
	mustBeNil(err)

	cfg := config.GetGlobalConfig()
	store, err := initStorage(cfg.Store, cfg.StorePath)
	mustBeNil(err)

	manager := scrape.NewManager()
}

func initStorage(store, storagePath string) (store.Storage, error) {
	switch store {
	case "badger":
		return badger.NewDB(storagePath)
	default:
		panic("unsupported storage " + store)
	}
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

func mustBeNil(err error) {
	if err == nil {
		return
	}
	fmt.Println(err.Error())
	os.Exit(-1)
}
