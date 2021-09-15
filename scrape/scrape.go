package scrape

import (
	"net/url"
	"sync"
	"time"
)

// Target refers to a singular HTTP or HTTPS endpoint.
type Target struct {
	job     string
	address string

	*url.URL

	mu                 sync.RWMutex
	lastError          error
	lastScrape         time.Time
	lastScrapeDuration time.Duration
}

func NewTarget(job, address string, u *url.URL) *Target {
	return &Target{
		job:     job,
		address: address,
		URL:     u,
	}
}

func (t *Target) BuildURL(schema, path string, header, params map[string]string) {
	vs := url.Values{}

	for k, v := range params {
		vs.Set(k, v)
	}

	t.URL = &url.URL{
		Scheme:   schema,
		Host:     t.address,
		Path:     path,
		RawQuery: vs.Encode(),
	}
}
