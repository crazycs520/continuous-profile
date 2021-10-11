package discovery

import (
	"context"
	"crypto/tls"
	"github.com/crazycs520/continuous-profile/util"
	"github.com/crazycs520/continuous-profile/util/logutil"
	"github.com/pingcap/tidb-dashboard/pkg/httpc"
	"github.com/pingcap/tidb-dashboard/pkg/pd"
	"github.com/pingcap/tidb-dashboard/pkg/utils/topology"
	"go.etcd.io/etcd/clientv3"
	"go.uber.org/zap"
	"sync"
	"time"
)

const discoverInterval = time.Second * 30

type TopologyDiscoverer struct {
	sync.Mutex
	PDClient   *pd.Client
	EtcdClient *clientv3.Client
	subscriber []chan []Component
	closed     chan struct{}
}

type Component struct {
	Name       string
	IP         string
	Port       uint
	StatusPort uint
}

type Subscriber = chan []Component

func NewTopologyDiscoverer(pdAddr string, tlsConfig *tls.Config) (*TopologyDiscoverer, error) {
	cfg := buildDashboardConfig(pdAddr, tlsConfig)
	lc := &mockLifecycle{}
	httpCli := httpc.NewHTTPClient(lc, cfg)
	pdCli := pd.NewPDClient(lc, httpCli, cfg)
	etcdCli, err := pd.NewEtcdClient(lc, cfg)
	if err != nil {
		return nil, err
	}
	d := &TopologyDiscoverer{
		PDClient:   pdCli,
		EtcdClient: etcdCli,
		closed:     make(chan struct{}),
	}
	return d, nil
}

func (d *TopologyDiscoverer) Subscribe() chan []Component {
	ch := make(chan []Component)
	d.Lock()
	d.subscriber = append(d.subscriber, ch)
	d.Unlock()
	return ch
}

func (d *TopologyDiscoverer) Start() {
	go util.GoWithRecovery(d.loadTopologyLoop, nil)
}

func (d *TopologyDiscoverer) Close() error {
	close(d.closed)
	return d.EtcdClient.Close()
}

func (d *TopologyDiscoverer) loadTopologyLoop() {
	d.loadTopology()
	ticker := time.NewTicker(discoverInterval)
	for {
		select {
		case <-d.closed:
			return
		case <-ticker.C:
			d.loadTopology()
		}
	}
}

func (d *TopologyDiscoverer) loadTopology() {
	ctx, cancel := context.WithTimeout(context.Background(), discoverInterval)
	defer cancel()
	components, err := d.getAllScrapeTargets(ctx)
	if err != nil {
		logutil.BgLogger().Error("load topology failed", zap.Error(err))
		return
	}
	d.notifySubscriber(components)
}

func (d *TopologyDiscoverer) notifySubscriber(components []Component) {
	for _, ch := range d.subscriber {
		select {
		case ch <- components:
		default:
		}
	}
}

func (d *TopologyDiscoverer) getAllScrapeTargets(ctx context.Context) ([]Component, error) {
	fns := []func(context.Context) ([]Component, error){
		d.getTiDBComponents,
		d.getPDComponents,
		d.getStoreComponents,
	}
	components := make([]Component, 0, 8)
	for _, fn := range fns {
		nodes, err := fn(ctx)
		if err != nil {
			return nil, err
		}
		components = append(components, nodes...)
	}
	return components, nil
}

func (d *TopologyDiscoverer) getTiDBComponents(ctx context.Context) ([]Component, error) {
	instances, err := topology.FetchTiDBTopology(ctx, d.EtcdClient)
	if err != nil {
		return nil, err
	}
	components := make([]Component, 0, len(instances))
	for _, instance := range instances {
		if instance.Status != topology.ComponentStatusUp {
			continue
		}
		components = append(components, Component{
			Name:       ComponentTiDB,
			IP:         instance.IP,
			Port:       instance.Port,
			StatusPort: instance.StatusPort,
		})
	}
	return components, nil
}

func (d *TopologyDiscoverer) getPDComponents(ctx context.Context) ([]Component, error) {
	instances, err := topology.FetchPDTopology(d.PDClient)
	if err != nil {
		return nil, err
	}
	components := make([]Component, 0, len(instances))
	for _, instance := range instances {
		if instance.Status != topology.ComponentStatusUp {
			continue
		}
		components = append(components, Component{
			Name:       ComponentTiDB,
			IP:         instance.IP,
			Port:       instance.Port,
			StatusPort: instance.Port,
		})
	}
	return components, nil
}

func (d *TopologyDiscoverer) getStoreComponents(ctx context.Context) ([]Component, error) {
	tikvInstances, tiflashInstances, err := topology.FetchStoreTopology(d.PDClient)
	if err != nil {
		return nil, err
	}
	components := make([]Component, 0, len(tikvInstances)+len(tiflashInstances))
	getComponents := func(instances []topology.StoreInfo) {
		for _, instance := range instances {
			if instance.Status != topology.ComponentStatusUp {
				continue
			}
			components = append(components, Component{
				Name:       ComponentTiDB,
				IP:         instance.IP,
				Port:       instance.Port,
				StatusPort: instance.StatusPort,
			})
		}
	}
	getComponents(tikvInstances)
	getComponents(tiflashInstances)
	return components, nil
}
