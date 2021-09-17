package scrape

import (
	"bytes"
	"context"
	"fmt"
	"github.com/crazycs520/continuous-profile/codec"
	"github.com/crazycs520/continuous-profile/config"
	"github.com/crazycs520/continuous-profile/store"
	"github.com/crazycs520/continuous-profile/util"
	"github.com/google/pprof/profile"
	"github.com/pkg/errors"
	"golang.org/x/net/context/ctxhttp"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

type ScrapeSuite struct {
	scraper        Scraper
	lastScrapeSize int

	store store.Storage

	ctx       context.Context
	scrapeCtx context.Context
	cancel    func()
	stopped   chan struct{}
}

func newScrapeSuite(ctx context.Context,
	sc Scraper,
	store store.Storage,
) *ScrapeSuite {
	sl := &ScrapeSuite{
		scraper: sc,
		store:   store,
		stopped: make(chan struct{}),
		ctx:     ctx,
	}
	sl.scrapeCtx, sl.cancel = context.WithCancel(ctx)

	return sl
}

func (sl *ScrapeSuite) run(interval, timeout time.Duration) {
	nextStart := time.Now().UnixNano() % int64(interval)
	select {
	case <-time.After(time.Duration(nextStart)):
		// Continue after a scraping offset.
	case <-sl.scrapeCtx.Done():
		close(sl.stopped)
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	buf := bytes.NewBuffer(make([]byte, 0, 1024))
	lastScrapeSize := 0

	for {
		//select {
		//case <-sl.ctx.Done():
		//	close(sl.stopped)
		//	return
		//case <-sl.scrapeCtx.Done():
		//	break mainLoop
		//default:
		//}

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
			}

			start.Nanosecond()
			ts := util.Millisecond(start)
			job := sl.scraper.target.job
			tp := sl.scraper.target.profileType
			instance := sl.scraper.target.address

			key := codec.ProfileKey{
				Ts:       ts,
				Job:      job,
				Tp:       tp,
				Instance: instance,
			}
			err := sl.store.Set(key.Encode(), buf.Bytes())
			if err != nil {
				//level.Debug(sl.l).Log("err", err)
			}

			//sl.target.health = HealthGood
			//sl.target.lastScrapeDuration = time.Since(start)
			//sl.target.lastError = nil
		} else {
			//level.Debug(sl.l).Log("msg", "Scrape failed", "err", scrapeErr.Error())

			//sl.target.health = HealthBad
			//sl.target.lastScrapeDuration = time.Since(start)
			//sl.target.lastError = scrapeErr
		}

		sl.scraper.target.lastScrape = start

		select {
		case <-sl.ctx.Done():
			close(sl.stopped)
			return
		case <-sl.scrapeCtx.Done():
			close(sl.stopped)
			return
		case <-ticker.C:
		}
	}
}

// Stop the scraping. May still write data and stale markers after it has
// returned. Cancel the context to stop all writes.
func (sl *ScrapeSuite) stop() {
	sl.cancel()
	<-sl.stopped
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

	p, err := profile.ParseData(b)
	if err != nil {
		return errors.Wrap(err, "failed to parse target's pprof profile")
	}

	if len(p.Sample) == 0 {
		return fmt.Errorf("empty %s profile from %s", s.target.profileType, s.req.URL.String())
	}

	if err := p.WriteUncompressed(w); err != nil {
		return fmt.Errorf("write profile: %w", err)
	}
	return nil
}

// Target refers to a singular HTTP or HTTPS endpoint.
type Target struct {
	job         string
	profileType string

	address string
	header  map[string]string
	*url.URL

	mu                 sync.RWMutex
	lastError          error
	lastScrape         time.Time
	lastScrapeDuration time.Duration
}

func NewTarget(job, schema, address, profileType string, cfg *config.PprofProfilingConfig) *Target {
	t := &Target{
		job:         job,
		profileType: profileType,
	}
	vs := url.Values{}
	for k, v := range cfg.Params {
		vs.Set(k, v)
	}
	if cfg.Seconds > 0 {
		vs.Add("seconds", strconv.Itoa(cfg.Seconds))
	}

	t.address = address
	t.header = cfg.Header
	t.URL = &url.URL{
		Scheme:   schema,
		Host:     t.address,
		Path:     cfg.Path,
		RawQuery: vs.Encode(),
	}
	return t
}

func (t *Target) GetURLString() string {
	return t.URL.String()
}
