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
	"sort"
	"strconv"
	"sync"
	"time"
)

// Manager maintains a set of scrape pools and manages start/stop cycles
// when receiving new target groups form the discovery manager.
type Manager struct {
	store          *store.ProfileStorage
	topoSubScribe  discovery.Subscriber
	reloadCh       chan struct{}
	curComponents  map[discovery.Component]struct{}
	lastComponents map[discovery.Component]struct{}

	cancel context.CancelFunc
	wg     sync.WaitGroup

	graceShut chan struct{}

	mtxScrape    sync.Mutex // Guards the fields below.
	scrapeSuites map[meta.ProfileTarget]*ScrapeSuite

	triggerReload chan struct{}
}

// NewManager is the Manager constructor
func NewManager(store *store.ProfileStorage, topoSubScribe discovery.Subscriber) *Manager {
	return &Manager{
		store:          store,
		topoSubScribe:  topoSubScribe,
		reloadCh:       make(chan struct{}, 10),
		curComponents:  map[discovery.Component]struct{}{},
		lastComponents: map[discovery.Component]struct{}{},
		scrapeSuites:   make(map[meta.ProfileTarget]*ScrapeSuite),
		graceShut:      make(chan struct{}),
		triggerReload:  make(chan struct{}, 1),
	}
}

func (m *Manager) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	go util.GoWithRecovery(func() {
		m.run(ctx)
	}, nil)
}

func (m *Manager) NotifyReload() {
	select {
	case m.reloadCh <- struct{}{}:
	default:
	}
}

func (m *Manager) GetCurrentScrapeComponents() []discovery.Component {
	components := make([]discovery.Component, 0, len(m.curComponents))
	for comp := range m.curComponents {
		components = append(components, comp)
	}
	sort.Slice(components, func(i, j int) bool {
		if components[i].Name != components[j].Name {
			return components[i].Name < components[j].Name
		}
		if components[i].IP != components[j].IP {
			return components[i].IP < components[j].IP
		}
		return components[i].Port < components[j].Port
	})
	return components
}

func (m *Manager) run(ctx context.Context) {
	buildMap := func(components []discovery.Component) map[discovery.Component]struct{} {
		m := make(map[discovery.Component]struct{}, len(components))
		for _, comp := range components {
			m[comp] = struct{}{}
		}
		return m
	}
	oldCfg := config.GetGlobalConfig().ContinueProfiling
	for {
		select {
		case <-ctx.Done():
			return
		case components := <-m.topoSubScribe:
			m.lastComponents = buildMap(components)
		case <-m.reloadCh:
			break
		}

		newCfg := config.GetGlobalConfig().ContinueProfiling
		m.reload(ctx, oldCfg, newCfg)
		oldCfg = newCfg
	}
}

func (m *Manager) reload(ctx context.Context, oldCfg, newCfg config.ContinueProfilingConfig) {
	configChanged := oldCfg != newCfg
	// close for old components
	for comp := range m.curComponents {
		_, exist := m.lastComponents[comp]
		if exist && !configChanged {
			continue
		}
		m.stopScrape(comp)
	}

	// close for old components
	if !newCfg.Enable {
		return
	}

	//start for new component.
	for comp := range m.lastComponents {
		_, exist := m.curComponents[comp]
		if exist && !configChanged {
			continue
		}
		err := m.startScrape(ctx, comp, newCfg)
		if err != nil {
			logutil.BgLogger().Error("start scrape failed",
				zap.String("component", comp.Name),
				zap.String("address", comp.IP+":"+strconv.Itoa(int(comp.StatusPort))))
		}
	}
}

func (m *Manager) startScrape(ctx context.Context, component discovery.Component, continueProfilingCfg config.ContinueProfilingConfig) error {
	if !continueProfilingCfg.Enable {
		return nil
	}
	profilingConfig := m.getProfilingConfig(component)
	cfg := config.GetGlobalConfig()
	httpCfg := cfg.Security.GetHTTPClientConfig()
	addr := fmt.Sprintf("%v:%v", component.IP, component.StatusPort)
	for profileName, profileConfig := range profilingConfig.PprofConfig {
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
	m.curComponents[component] = struct{}{}
	logutil.BgLogger().Info("start component scrape",
		zap.String("component", component.Name),
		zap.String("address", addr))
	return nil
}

func (m *Manager) stopScrape(component discovery.Component) {
	delete(m.curComponents, component)
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
	return &config.ProfilingConfig{
		PprofConfig: config.PprofConfig{
			"allocs": &config.PprofProfilingConfig{
				Path: "/debug/pprof/allocs",
			},
			"goroutine": &config.PprofProfilingConfig{
				Path:   "/debug/pprof/goroutine",
				Params: map[string]string{"debug": "2"},
			},
			"mutex": &config.PprofProfilingConfig{
				Path: "/debug/pprof/mutex",
			},
			"profile": &config.PprofProfilingConfig{
				Path:    "/debug/pprof/profile",
				Seconds: cfg.ProfileSeconds,
			},
		},
	}
}

func nonGoAppProfilingConfig() *config.ProfilingConfig {
	cfg := config.GetGlobalConfig().ContinueProfiling
	return &config.ProfilingConfig{
		PprofConfig: config.PprofConfig{
			"profile": &config.PprofProfilingConfig{
				Path:    "/debug/pprof/profile",
				Seconds: cfg.ProfileSeconds,
				Header:  map[string]string{"Content-Type": "application/protobuf"},
			},
		},
	}
}
