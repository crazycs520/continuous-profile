package discovery

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/crazycs520/continuous-profile/config"
	dashboard_config "github.com/pingcap/tidb-dashboard/pkg/config"
	"github.com/pingcap/tidb-dashboard/pkg/httpc"
	"github.com/pingcap/tidb-dashboard/pkg/pd"
	"github.com/pingcap/tidb-dashboard/pkg/utils/topology"
	"github.com/prometheus/common/model"
	"go.etcd.io/etcd/clientv3"
	"go.uber.org/fx"
)

const (
	ComponentTiDB    = "tidb"
	ComponentTiKV    = "tikv"
	ComponentTiFlash = "tiflash"
	ComponentPD      = "pd"
)

type DiscoveryClient struct {
	PDClient   *pd.Client
	EtcdClient *clientv3.Client
}

func NewDiscoveryClient(pdAddr string, tlsConfig *tls.Config) (*DiscoveryClient, error) {
	cfg := buildDashboardConfig(pdAddr, tlsConfig)
	lc := &mockLifecycle{}
	httpCli := httpc.NewHTTPClient(lc, cfg)
	pdCli := pd.NewPDClient(lc, httpCli, cfg)
	etcdCli, err := pd.NewEtcdClient(lc, cfg)
	if err != nil {
		return nil, err
	}
	return &DiscoveryClient{
		PDClient:   pdCli,
		EtcdClient: etcdCli,
	}, nil
}

func (d *DiscoveryClient) GetAllScrapeTargets(ctx context.Context) ([]*config.ScrapeConfig, error) {
	tidb, err := d.getTiDBScrapeTargets(ctx)
	if err != nil {
		return nil, err
	}
	pd, err := d.getPDScrapeTargets()
	if err != nil {
		return nil, err
	}
	tikv, tiflash, err := d.getStoreScrapeTargets()
	if err != nil {
		return nil, err
	}
	return []*config.ScrapeConfig{tidb, pd, tikv, tiflash}, nil
}

func (d *DiscoveryClient) Close() error {
	return d.EtcdClient.Close()
}

func (d *DiscoveryClient) getTiDBScrapeTargets(ctx context.Context) (*config.ScrapeConfig, error) {
	instances, err := topology.FetchTiDBTopology(ctx, d.EtcdClient)
	if err != nil {
		return nil, err
	}
	scrapeConfig := d.newScrapeTargets(ComponentTiDB, GoAppProfilingConfig())
	targets := make([]string, 0, len(instances))
	for _, instance := range instances {
		if instance.Status != topology.ComponentStatusUp {
			continue
		}
		addr := fmt.Sprintf("%v:%v", instance.IP, instance.StatusPort)
		targets = append(targets, addr)
	}
	scrapeConfig.Targets = targets
	return scrapeConfig, nil
}

func (d *DiscoveryClient) getPDScrapeTargets() (*config.ScrapeConfig, error) {
	instances, err := topology.FetchPDTopology(d.PDClient)
	if err != nil {
		return nil, err
	}
	scrapeConfig := d.newScrapeTargets(ComponentPD, GoAppProfilingConfig())
	targets := make([]string, 0, len(instances))
	for _, instance := range instances {
		if instance.Status != topology.ComponentStatusUp {
			continue
		}
		addr := fmt.Sprintf("%v:%v", instance.IP, instance.Port)
		targets = append(targets, addr)
	}
	scrapeConfig.Targets = targets
	return scrapeConfig, nil
}

func (d *DiscoveryClient) getStoreScrapeTargets() (*config.ScrapeConfig, *config.ScrapeConfig, error) {
	tikvInstances, tiflashInstances, err := topology.FetchStoreTopology(d.PDClient)
	if err != nil {
		return nil, nil, err
	}
	buildTargets := func(instances []topology.StoreInfo) []string {
		targets := make([]string, 0, len(instances))
		for _, instance := range instances {
			if instance.Status != topology.ComponentStatusUp {
				continue
			}
			addr := fmt.Sprintf("%v:%v", instance.IP, instance.StatusPort)
			targets = append(targets, addr)
		}
		return targets
	}
	tikvScrapeConfig := d.newScrapeTargets(ComponentTiKV, NonGoAppProfilingConfig())
	tikvScrapeConfig.Targets = buildTargets(tikvInstances)
	tiflashScrapeConfig := d.newScrapeTargets(ComponentTiFlash, NonGoAppProfilingConfig())
	tiflashScrapeConfig.Targets = buildTargets(tiflashInstances)
	return tikvScrapeConfig, tiflashScrapeConfig, nil
}

func (d *DiscoveryClient) newScrapeTargets(component string, profiling *config.ProfilingConfig) *config.ScrapeConfig {
	cfg := config.GetGlobalConfig()
	return &config.ScrapeConfig{
		ComponentName:   component,
		ScrapeInterval:  secondToDuration(cfg.ContinueProfiling.IntervalSeconds),
		ScrapeTimeout:   secondToDuration(cfg.ContinueProfiling.TimeoutSeconds),
		Scheme:          cfg.GetHTTPScheme(),
		ProfilingConfig: profiling,
	}
}

func GoAppProfilingConfig() *config.ProfilingConfig {
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

func NonGoAppProfilingConfig() *config.ProfilingConfig {
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

func secondToDuration(secs int) model.Duration {
	return model.Duration(time.Duration(secs) * time.Second)
}

func buildDashboardConfig(pdAddr string, tlsConfig *tls.Config) *dashboard_config.Config {
	return &dashboard_config.Config{
		PDEndPoint:       fmt.Sprintf("http://%v", pdAddr),
		ClusterTLSConfig: tlsConfig,
	}
}

type mockLifecycle struct{}

func (_ *mockLifecycle) Append(fx.Hook) {
	return
}
