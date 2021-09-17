package scrape

import (
	"context"
	"github.com/crazycs520/continuous-profile/config"
	"github.com/crazycs520/continuous-profile/util"
	"github.com/dgraph-io/badger/v3"
	commonconfig "github.com/prometheus/common/config"
	"sync"
	"time"
)

// Manager maintains a set of scrape pools and manages start/stop cycles
// when receiving new target groups form the discovery manager.
type Manager struct {
	db        *badger.DB
	graceShut chan struct{}

	mtxScrape    sync.Mutex // Guards the fields below.
	scrapeSuites map[scrapeTargetKey]*ScrapeSuite

	triggerReload chan struct{}
}

// NewManager is the Manager constructor
func NewManager(db *badger.DB) *Manager {
	return &Manager{
		db:            db,
		scrapeSuites:  make(map[scrapeTargetKey]*ScrapeSuite),
		graceShut:     make(chan struct{}),
		triggerReload: make(chan struct{}, 1),
	}
}

func (m *Manager) InitScrape(ctx context.Context, db *badger.DB, scrapeConfigs []config.ScrapeConfig) error {
	for _, scfg := range scrapeConfigs {
		for _, addr := range scfg.Targets {
			for profileName, profileConfig := range scfg.ProfilingConfig.PprofConfig {
				if *profileConfig.Enabled == false {
					continue
				}
				target := NewTarget(scfg.JobName, scfg.Scheme, addr, profileName, profileConfig)
				client, err := commonconfig.NewClientFromConfig(scfg.HTTPClientConfig, scfg.JobName)
				if err != nil {
					return err
				}
				scrape := newScraper(target, client)
				scrapeSuite := newScrapeSuite(ctx, scrape, db)
				key := scrapeTargetKey{
					job:         scfg.JobName,
					address:     addr,
					profileType: profileName,
				}

				interval := time.Duration(scfg.ScrapeInterval)
				timeout := time.Duration(scfg.ScrapeTimeout)
				go util.GoWithRecovery(func() {
					scrapeSuite.run(interval, timeout)
				}, nil)
				m.scrapeSuites[key] = scrapeSuite
			}
		}
	}
	return nil
}

type scrapeTargetKey struct {
	job         string
	address     string
	profileType string
}
