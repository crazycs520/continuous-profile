package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/crazycs520/continuous-profile/meta"
	"github.com/crazycs520/continuous-profile/util"
	"github.com/crazycs520/continuous-profile/util/logutil"
	"github.com/dgraph-io/badger/v3"
	"github.com/dgraph-io/badger/v3/options"
	"github.com/genjidb/genji"
	"github.com/genjidb/genji/document"
	"github.com/genjidb/genji/engine/badgerengine"
	"github.com/genjidb/genji/types"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"sync"
	"time"
)

const (
	tableNamePrefix = "continuous_profiling"
	metaTableName   = tableNamePrefix + "_targets_meta"
)

var ErrStoreIsClosed = errors.New("storage is closed")

type ProfileStorage struct {
	closed atomic.Bool
	sync.Mutex
	db          *genji.DB
	metaCache   map[meta.ProfileTarget]meta.TargetInfo
	idAllocator int64
}

func NewProfileStorage(storagePath string) (*ProfileStorage, error) {
	opts := badger.DefaultOptions(storagePath).
		WithCompression(options.ZSTD).
		WithZSTDCompressionLevel(3).
		WithBlockSize(8 * 1024 * 1024).
		WithValueThreshold(8 * 1024 * 1024).
		WithLogger(logutil.BadgerLogger())
	ng, err := badgerengine.NewEngine(opts)
	if err != nil {
		return nil, err
	}
	db, err := genji.New(context.Background(), ng)
	if err != nil {
		return nil, err
	}
	store := &ProfileStorage{
		db:        db,
		metaCache: make(map[meta.ProfileTarget]meta.TargetInfo),
	}
	err = store.init()
	if err != nil {
		return nil, err
	}

	go util.GoWithRecovery(store.doGCLoop, nil)

	return store, nil
}

func (s *ProfileStorage) init() error {
	err := s.initMetaTable()
	if err != nil {
		return err
	}
	err = s.loadMetaIntoCache()
	if err != nil {
		return err
	}
	return nil
}

func (s *ProfileStorage) initMetaTable() error {
	// create meta table if not exists.
	sql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %v (id INTEGER primary key, kind TEXT, component TEXT, address TEXT, last_scrape_ts INTEGER)", metaTableName)
	return s.db.Exec(sql)
}

func (s *ProfileStorage) loadMetaIntoCache() error {
	query := fmt.Sprintf("SELECT id, kind, component, address, last_scrape_ts FROM %v", metaTableName)
	res, err := s.db.Query(query)
	if err != nil {
		return err
	}
	defer res.Close()

	err = res.Iterate(func(d types.Document) error {
		var id, ts int64
		var kind, component, address string
		err = document.Scan(d, &id, &kind, &component, &address, &ts)
		if err != nil {
			return err
		}
		s.rebaseID(id)
		target := meta.ProfileTarget{
			Kind:      kind,
			Component: component,
			Address:   address,
		}
		s.metaCache[target] = meta.TargetInfo{
			ID:           id,
			LastScrapeTs: ts,
		}
		logutil.BgLogger().Info("load target info into cache",
			zap.String("component", target.Component),
			zap.String("address", target.Address),
			zap.String("kind", target.Kind),
			zap.Int64("id", id),
			zap.Int64("ts", ts))
		return nil
	})
	return err
}

func (s *ProfileStorage) AddProfile(pt meta.ProfileTarget, ts int64, profile []byte) error {
	if s.isClose() {
		return ErrStoreIsClosed
	}
	info, err := s.prepareProfileTable(pt)
	if err != nil {
		return err
	}

	sql := fmt.Sprintf("INSERT INTO %v (ts, data) VALUES (?, ?)", s.getProfileTableName(info))
	return s.db.Exec(sql, ts, profile)
}

func (s *ProfileStorage) QueryProfileList(param *meta.BasicQueryParam) ([]meta.ProfileList, error) {
	if s.isClose() {
		return nil, ErrStoreIsClosed
	}
	if param == nil {
		return nil, nil
	}
	targets := param.Targets
	if len(targets) == 0 {
		targets = s.getAllTargets()
	}

	var result []meta.ProfileList
	args := []interface{}{param.Begin, param.End}
	for _, pt := range targets {
		info, exist := s.getTargetInfoFromCache(pt)
		if !exist {
			result = append(result, meta.ProfileList{
				Target: pt,
			})
			continue
		}

		query := fmt.Sprintf("SELECT ts FROM %v WHERE ts >= ? and ts <= ?", s.getProfileTableName(info))
		res, err := s.db.Query(query, args...)
		if err != nil {
			return nil, err
		}
		var tsList []int64
		err = res.Iterate(func(d types.Document) error {
			var ts int64
			err = document.Scan(d, &ts)
			if err != nil {
				return err
			}
			tsList = append(tsList, ts)
			return nil
		})
		if err != nil {
			res.Close()
			return nil, err
		}
		err = res.Close()
		if err != nil {
			return nil, err
		}
		result = append(result, meta.ProfileList{
			Target: pt,
			TsList: tsList,
		})
	}
	return result, nil
}

func (s *ProfileStorage) QueryProfileData(param *meta.BasicQueryParam, handleFn func(meta.ProfileTarget, int64, []byte) error) error {
	if s.isClose() {
		return ErrStoreIsClosed
	}
	if param == nil || handleFn == nil {
		return nil
	}
	targets := param.Targets
	if len(targets) == 0 {
		targets = s.getAllTargets()
	}

	args := []interface{}{param.Begin, param.End}
	for _, pt := range targets {
		info, exist := s.getTargetInfoFromCache(pt)
		if !exist {
			continue
		}
		query := fmt.Sprintf("SELECT ts, data FROM %v WHERE ts >= ? and ts <= ?", s.getProfileTableName(info))
		res, err := s.db.Query(query, args...)
		if err != nil {
			return err
		}
		err = res.Iterate(func(d types.Document) error {
			var ts int64
			var data []byte
			err = document.Scan(d, &ts, &data)
			if err != nil {
				return err
			}
			return handleFn(pt, ts, data)
		})
		if err != nil {
			res.Close()
			return err
		}
		err = res.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *ProfileStorage) getTargetInfoFromCache(pt meta.ProfileTarget) (meta.TargetInfo, bool) {
	s.Lock()
	info, ok := s.metaCache[pt]
	s.Unlock()
	return info, ok
}

func (s *ProfileStorage) getAllTargets() []meta.ProfileTarget {
	s.Lock()
	defer s.Unlock()
	targets := make([]meta.ProfileTarget, 0, len(s.metaCache))
	for pt := range s.metaCache {
		targets = append(targets, pt)
	}
	return targets
}

func (s *ProfileStorage) Close() error {
	if s.isClose() {
		return nil
	}
	s.closed.Store(true)
	return s.db.Close()
}

func (s *ProfileStorage) isClose() bool {
	return s.closed.Load()
}

func (s *ProfileStorage) prepareProfileTable(pt meta.ProfileTarget) (meta.TargetInfo, error) {
	s.Lock()
	defer s.Unlock()
	info, ok := s.metaCache[pt]
	if ok {
		return info, nil
	}
	var err error
	info, err = s.createProfileTable(pt)
	if err != nil {
		return info, err
	}
	// update cache
	s.metaCache[pt] = info
	return info, nil
}

func (s *ProfileStorage) createProfileTable(pt meta.ProfileTarget) (meta.TargetInfo, error) {
	info := meta.TargetInfo{
		ID:           s.allocID(),
		LastScrapeTs: time.Now().Unix(),
	}
	sql := fmt.Sprintf("INSERT INTO %v (id, kind, component, address, last_scrape_ts) VALUES (?, ?, ?, ?, ?)", metaTableName)
	err := s.db.Exec(sql, info.ID, pt.Kind, pt.Component, pt.Address, info.LastScrapeTs)
	if err != nil {
		return info, err
	}
	tbName := s.getProfileTableName(info)
	sql = fmt.Sprintf("CREATE TABLE IF NOT EXISTS %v (ts INTEGER PRIMARY KEY, data BLOB)", tbName)
	err = s.db.Exec(sql)
	return info, err
}

func (s *ProfileStorage) getProfileTableName(info meta.TargetInfo) string {
	return fmt.Sprintf("`%v_%v`", tableNamePrefix, info.ID)
}

func (s *ProfileStorage) rebaseID(id int64) {
	if id <= s.idAllocator {
		return
	}
	s.idAllocator = id
}

func (s *ProfileStorage) allocID() int64 {
	s.idAllocator += 1
	return s.idAllocator
}
