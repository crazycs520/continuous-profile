package scrape

import (
	"context"
	"sync"
	"time"

	"github.com/crazycs520/continuous-profile/config"
	"github.com/crazycs520/continuous-profile/discovery"
	"github.com/crazycs520/continuous-profile/store"
	"github.com/crazycs520/continuous-profile/util"
	commonconfig "github.com/prometheus/common/config"
)

// Manager maintains a set of scrape pools and manages start/stop cycles
// when receiving new target groups form the discovery manager.
type Manager struct {
	store        *store.ProfileStorage
	discoveryCli *discovery.DiscoveryClient
	cancel       context.CancelFunc
	wg           sync.WaitGroup

	graceShut chan struct{}

	mtxScrape    sync.Mutex // Guards the fields below.
	scrapeSuites map[scrapeTargetKey]*ScrapeSuite

	triggerReload chan struct{}
}

// NewManager is the Manager constructor
func NewManager(store *store.ProfileStorage, discoveryCli *discovery.DiscoveryClient) *Manager {
	return &Manager{
		store:         store,
		discoveryCli:  discoveryCli,
		scrapeSuites:  make(map[scrapeTargetKey]*ScrapeSuite),
		graceShut:     make(chan struct{}),
		triggerReload: make(chan struct{}, 1),
	}
}

func (m *Manager) InitScrape() error {
	var err error
	ctx, cancel := context.WithCancel(context.Background())
	cfg := config.GetGlobalConfig()
	scrapeConfigs := cfg.ScrapeConfigs
	if m.discoveryCli != nil {
		scrapeConfigs, err = m.discoveryCli.GetAllScrapeTargets(ctx)
		if err != nil {
			return err
		}
	}
	m.cancel = cancel
	for _, scfg := range scrapeConfigs {
		for _, addr := range scfg.Targets {
			for profileName, profileConfig := range scfg.ProfilingConfig.PprofConfig {
				if *profileConfig.Enabled == false {
					continue
				}
				target := NewTarget(scfg.ComponentName, addr, profileName, scfg.Scheme, profileConfig)
				client, err := commonconfig.NewClientFromConfig(scfg.HTTPClientConfig, scfg.ComponentName)
				if err != nil {
					return err
				}
				scrape := newScraper(target, client)
				scrapeSuite := newScrapeSuite(ctx, scrape, m.store)
				key := scrapeTargetKey{
					job:         scfg.ComponentName,
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
	err := m.store.Close()
	if err != nil {
		return err
	}
	m.wg.Wait()
	return nil
}

type scrapeTargetKey struct {
	job         string
	address     string
	profileType string
}
