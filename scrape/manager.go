package scrape

import (
	"context"
	"github.com/crazycs520/continuous-profile/config"
	"github.com/crazycs520/continuous-profile/store"
	"github.com/crazycs520/continuous-profile/util"
	commonconfig "github.com/prometheus/common/config"
	"sync"
	"time"
)

// Manager maintains a set of scrape pools and manages start/stop cycles
// when receiving new target groups form the discovery manager.
type Manager struct {
	store  *store.ProfileStorage
	cancel context.CancelFunc
	wg     sync.WaitGroup

	graceShut chan struct{}

	mtxScrape    sync.Mutex // Guards the fields below.
	scrapeSuites map[scrapeTargetKey]*ScrapeSuite

	triggerReload chan struct{}
}

// NewManager is the Manager constructor
func NewManager(store *store.ProfileStorage) *Manager {
	return &Manager{
		store:         store,
		scrapeSuites:  make(map[scrapeTargetKey]*ScrapeSuite),
		graceShut:     make(chan struct{}),
		triggerReload: make(chan struct{}, 1),
	}
}

func (m *Manager) InitScrape(scrapeConfigs []config.ScrapeConfig) error {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
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
				scrapeSuite := newScrapeSuite(ctx, scrape, m.store)
				key := scrapeTargetKey{
					job:         scfg.JobName,
					address:     addr,
					profileType: profileName,
				}

				interval := time.Duration(scfg.ScrapeInterval)
				timeout := time.Duration(scfg.ScrapeTimeout)
				m.wg.Add(1)
				go util.GoWithRecovery(func() {
					defer m.wg.Done()
					scrapeSuite.run(interval, timeout)
				}, nil)
				m.scrapeSuites[key] = scrapeSuite
			}
		}
	}
	return nil
}

func (m *Manager) Close() error {
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()
	return m.store.Close()
}

type scrapeTargetKey struct {
	job         string
	address     string
	profileType string
}
