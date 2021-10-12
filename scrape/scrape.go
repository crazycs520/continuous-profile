package scrape

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/crazycs520/continuous-profile/config"
	"github.com/crazycs520/continuous-profile/meta"
	"github.com/crazycs520/continuous-profile/store"
	"github.com/crazycs520/continuous-profile/util"
	"github.com/crazycs520/continuous-profile/util/logutil"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/net/context/ctxhttp"
)

type ScrapeSuite struct {
	scraper        Scraper
	lastScrapeSize int

	store *store.ProfileStorage

	ctx       context.Context
	scrapeCtx context.Context
	cancel    func()
}

func newScrapeSuite(ctx context.Context, sc Scraper, store *store.ProfileStorage) *ScrapeSuite {
	sl := &ScrapeSuite{
		scraper: sc,
		store:   store,
		ctx:     ctx,
	}
	sl.scrapeCtx, sl.cancel = context.WithCancel(ctx)

	return sl
}

func (sl *ScrapeSuite) run(interval, timeout time.Duration) {
	target := sl.scraper.target
	logutil.BgLogger().Info("scraper start to run",
		zap.String("component", target.Component),
		zap.String("address", target.Address),
		zap.String("kind", target.Kind))
	nextStart := time.Now().UnixNano() % int64(interval)
	select {
	case <-time.After(time.Duration(nextStart)):
		// Continue after a scraping offset.
	case <-sl.scrapeCtx.Done():
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	buf := bytes.NewBuffer(make([]byte, 0, 1024))
	lastScrapeSize := 0

	for {
		start := time.Now()
		if lastScrapeSize > 0 && buf.Cap() > 2*lastScrapeSize {
			// shrink the buffer size.
			buf = bytes.NewBuffer(make([]byte, 0, lastScrapeSize))
		}

		buf.Reset()

		scrapeCtx, cancel := context.WithTimeout(sl.ctx, timeout)
		scrapeErr := sl.scraper.scrape(scrapeCtx, buf)
		cancel()

		if scrapeErr == nil {
			if buf.Len() > 0 {
				lastScrapeSize = buf.Len()
				ts := util.GetTimeStamp(start)
				err := sl.store.AddProfile(meta.ProfileTarget{
					Kind:      sl.scraper.target.Kind,
					Component: sl.scraper.target.Component,
					Address:   sl.scraper.target.Address,
				}, ts, buf.Bytes())

				if err == nil {
					sl.scraper.target.lastScrape = start
				} else {
					logutil.BgLogger().Error("save scrape data failed",
						zap.String("component", target.Component),
						zap.String("address", target.Address),
						zap.String("kind", target.Kind),
						zap.Error(err))
				}
			}
		} else {
			logutil.BgLogger().Error("scrape failed",
				zap.String("component", target.Component),
				zap.String("address", target.Address),
				zap.String("kind", target.Kind),
				zap.Error(scrapeErr))
		}

		select {
		case <-sl.ctx.Done():
			return
		case <-sl.scrapeCtx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Stop the scraping. May still write data and stale markers after it has
// returned. Cancel the context to stop all writes.
func (sl *ScrapeSuite) stop() {
	sl.cancel()
}

type Scraper struct {
	target *Target
	client *http.Client
	req    *http.Request
}

func newScraper(target *Target, client *http.Client) Scraper {
	return Scraper{
		target: target,
		client: client,
	}
}

func (s *Scraper) scrape(ctx context.Context, w io.Writer) error {
	cfg := config.GetGlobalConfig()
	if !cfg.ContinueProfiling.Enable {
		return nil
	}

	if s.req == nil {
		req, err := http.NewRequest("GET", s.target.GetURLString(), nil)
		if err != nil {
			return err
		}
		if header := s.target.header; len(header) > 0 {
			for k, v := range header {
				req.Header.Set(k, v)
			}
		}

		s.req = req
	}

	resp, err := ctxhttp.Do(ctx, s.client, s.req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned HTTP status %s", resp.Status)
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read body")
	}

	data := s.tryUnzip(b)
	_, err = w.Write(data)
	return err
}

func (s *Scraper) tryUnzip(data []byte) []byte {
	gz, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return data
	}
	v, err := ioutil.ReadAll(gz)
	if err != nil {
		return data
	}
	return v
}

// Target refers to a singular HTTP or HTTPS endpoint.
type Target struct {
	meta.ProfileTarget

	header map[string]string
	*url.URL

	mu                 sync.RWMutex
	lastError          error
	lastScrape         time.Time
	lastScrapeDuration time.Duration
}

func NewTarget(component, address, kind, schema string, cfg *config.PprofProfilingConfig) *Target {
	t := &Target{
		ProfileTarget: meta.ProfileTarget{
			Kind:      kind,
			Component: component,
			Address:   address,
		},
	}
	vs := url.Values{}
	for k, v := range cfg.Params {
		vs.Set(k, v)
	}
	if cfg.Seconds > 0 {
		vs.Add("seconds", strconv.Itoa(cfg.Seconds))
	}

	t.header = cfg.Header
	t.URL = &url.URL{
		Scheme:   schema,
		Host:     t.Address,
		Path:     cfg.Path,
		RawQuery: vs.Encode(),
	}
	return t
}

func (t *Target) GetURLString() string {
	return t.URL.String()
}
