package scrape

import (
	"context"
	"fmt"
	"github.com/crazycs520/continuous-profile/config"
	"github.com/crazycs520/continuous-profile/discovery"
	"github.com/crazycs520/continuous-profile/meta"
	"github.com/crazycs520/continuous-profile/store"
	"github.com/crazycs520/continuous-profile/util"
	"github.com/crazycs520/continuous-profile/util/logutil"
	commonconfig "github.com/prometheus/common/config"
	"go.uber.org/zap"
	"strconv"
	"sync"
	"time"
)

// Manager maintains a set of scrape pools and manages start/stop cycles
// when receiving new target groups form the discovery manager.
type Manager struct {
	store         *store.ProfileStorage
	topoSubScribe discovery.Subscriber
	cancel        context.CancelFunc
	wg            sync.WaitGroup

	graceShut chan struct{}

	mtxScrape    sync.Mutex // Guards the fields below.
	scrapeSuites map[meta.ProfileTarget]*ScrapeSuite

	triggerReload chan struct{}
}

// NewManager is the Manager constructor
func NewManager(store *store.ProfileStorage, topoSubScribe discovery.Subscriber) *Manager {
	return &Manager{
		store:         store,
		topoSubScribe: topoSubScribe,
		scrapeSuites:  make(map[meta.ProfileTarget]*ScrapeSuite),
		graceShut:     make(chan struct{}),
		triggerReload: make(chan struct{}, 1),
	}
}

func (m *Manager) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	go util.GoWithRecovery(func() {
		m.run(ctx)
	}, nil)
}

func (m *Manager) run(ctx context.Context) {
	buildMap := func(components []discovery.Component) map[discovery.Component]struct{} {
		m := make(map[discovery.Component]struct{}, len(components))
		for _, comp := range components {
			m[comp] = struct{}{}
		}
		return m
	}
	var oldMap map[discovery.Component]struct{}
	oldContinueProfilingCfg := config.GetGlobalConfig().ContinueProfiling
	for {
		components := <-m.topoSubScribe
		newMap := buildMap(components)

		newContinueProfilingCfg := config.GetGlobalConfig().ContinueProfiling
		configChanged := oldContinueProfilingCfg != newContinueProfilingCfg
		// close for old components

		for comp := range oldMap {
			_, exist := newMap[comp]
			if exist && !configChanged {
				continue
			}
			m.stopScrape(comp)
		}

		//start for new component.
		for comp := range newMap {
			_, exist := oldMap[comp]
			if exist {
				continue
			}
			err := m.startScrape(ctx, comp, newContinueProfilingCfg)
			if err != nil {
				logutil.BgLogger().Error("start scrape failed",
					zap.String("component", comp.Name),
					zap.String("address", comp.IP+":"+strconv.Itoa(int(comp.StatusPort))))
			}
		}

	}
}

func (m *Manager) startScrape(ctx context.Context, component discovery.Component, continueProfilingCfg config.ContinueProfilingConfig) error {
	profilingConfig := m.getProfilingConfig(component)
	cfg := config.GetGlobalConfig()
	httpCfg := cfg.Security.GetHTTPClientConfig()
	addr := fmt.Sprintf("%v:%v", component.IP, component.StatusPort)
	for profileName, profileConfig := range profilingConfig.PprofConfig {
		if *profileConfig.Enabled == false {
			continue
		}
		target := NewTarget(component.Name, addr, profileName, cfg.GetHTTPScheme(), profileConfig)
		client, err := commonconfig.NewClientFromConfig(httpCfg, component.Name)
		if err != nil {
			return err
		}
		scrape := newScraper(target, client)
		scrapeSuite := newScrapeSuite(ctx, scrape, m.store)
		key := meta.ProfileTarget{
			Kind:      profileName,
			Component: component.Name,
			Address:   addr,
		}

		interval := time.Duration(continueProfilingCfg.IntervalSeconds) * time.Second
		timeout := time.Duration(continueProfilingCfg.TimeoutSeconds) * time.Second
		m.wg.Add(1)
		go util.GoWithRecovery(func() {
			defer m.wg.Done()
			scrapeSuite.run(interval, timeout)
		}, nil)
		m.scrapeSuites[key] = scrapeSuite
	}
	logutil.BgLogger().Info("start component scrape",
		zap.String("component", component.Name),
		zap.String("address", addr))
	return nil
}

func (m *Manager) stopScrape(component discovery.Component) {
	addr := fmt.Sprintf("%v:%v", component.IP, component.StatusPort)
	logutil.BgLogger().Info("stop component scrape",
		zap.String("component", component.Name),
		zap.String("address", addr))
	profilingConfig := m.getProfilingConfig(component)
	for profileName := range profilingConfig.PprofConfig {
		key := meta.ProfileTarget{
			Kind:      profileName,
			Component: component.Name,
			Address:   addr,
		}
		scrapeSuite, ok := m.scrapeSuites[key]
		if !ok {
			continue
		}
		scrapeSuite.stop()
	}
}

func (m *Manager) getProfilingConfig(component discovery.Component) *config.ProfilingConfig {
	switch component.Name {
	case discovery.ComponentTiDB, discovery.ComponentPD:
		return goAppProfilingConfig()
	default:
		return nonGoAppProfilingConfig()
	}
}

//func (m *Manager) InitScrape() error {
//	ctx, cancel := context.WithCancel(context.Background())
//	if m.discoveryCli == nil {
//		return nil
//	}
//	scrapeConfigs, err := m.discoveryCli.GetAllScrapeTargets(ctx)
//	if err != nil {
//		return err
//	}
//	m.cancel = cancel
//	cfg := config.GetGlobalConfig()
//	httpCfg := cfg.Security.GetHTTPClientConfig()
//	for _, scfg := range scrapeConfigs {
//		for _, addr := range scfg.Targets {
//			for profileName, profileConfig := range scfg.ProfilingConfig.PprofConfig {
//				if *profileConfig.Enabled == false {
//					continue
//				}
//				target := NewTarget(scfg.ComponentName, addr, profileName, cfg.GetHTTPScheme(), profileConfig)
//				client, err := commonconfig.NewClientFromConfig(httpCfg, scfg.ComponentName)
//				if err != nil {
//					return err
//				}
//				scrape := newScraper(target, client)
//				scrapeSuite := newScrapeSuite(ctx, scrape, m.store)
//				key := scrapeTargetKey{
//					component:   scfg.ComponentName,
//					address:     addr,
//					profileType: profileName,
//				}
//
//				interval := scfg.ScrapeInterval
//				timeout := scfg.ScrapeTimeout
//				m.wg.Add(1)
//				go util.GoWithRecovery(func() {
//					defer m.wg.Done()
//					scrapeSuite.run(interval, timeout)
//				}, nil)
//				m.scrapeSuites[key] = scrapeSuite
//			}
//		}
//	}
//	return nil
//}

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

func goAppProfilingConfig() *config.ProfilingConfig {
	cfg := config.GetGlobalConfig().ContinueProfiling
	trueValue := true
	return &config.ProfilingConfig{
		PprofConfig: config.PprofConfig{
			"allocs": &config.PprofProfilingConfig{
				Enabled: &trueValue,
				Path:    "/debug/pprof/allocs",
			},
			"goroutine": &config.PprofProfilingConfig{
				Enabled: &trueValue,
				Path:    "/debug/pprof/goroutine",
				Params:  map[string]string{"debug": "2"},
			},
			"mutex": &config.PprofProfilingConfig{
				Enabled: &trueValue,
				Path:    "/debug/pprof/mutex",
			},
			"profile": &config.PprofProfilingConfig{
				Enabled: &trueValue,
				Path:    "/debug/pprof/profile",
				Seconds: cfg.ProfileSeconds,
			},
		},
	}
}

func nonGoAppProfilingConfig() *config.ProfilingConfig {
	cfg := config.GetGlobalConfig().ContinueProfiling
	trueValue := true
	return &config.ProfilingConfig{
		PprofConfig: config.PprofConfig{
			"profile": &config.PprofProfilingConfig{
				Enabled: &trueValue,
				Path:    "/debug/pprof/profile",
				Seconds: cfg.ProfileSeconds,
				Header:  map[string]string{"Content-Type": "application/protobuf"},
			},
		},
	}
}
