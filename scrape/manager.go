package scrape

import (
	"github.com/crazycs520/continuous-profile/config"
	"github.com/dgraph-io/badger/v3"
	"sync"
)

// Manager maintains a set of scrape pools and manages start/stop cycles
// when receiving new target groups form the discovery manager.
type Manager struct {
	logger    log.Logger
	db        *badger.DB
	graceShut chan struct{}

	mtxScrape     sync.Mutex // Guards the fields below.
	scrapeConfigs map[string]*config.ScrapeConfig
	scrapePools   map[string]*ScrapeSuite
	targetSets    map[string][]*targetgroup.Group

	triggerReload chan struct{}
}
