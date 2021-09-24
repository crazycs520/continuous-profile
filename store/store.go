package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/crazycs520/continuous-profile/meta"
	"github.com/crazycs520/continuous-profile/util/logutil"
	"github.com/dgraph-io/badger/v3"
	"github.com/dgraph-io/badger/v3/options"
	"github.com/genjidb/genji"
	"github.com/genjidb/genji/document"
	"github.com/genjidb/genji/engine/badgerengine"
	"github.com/genjidb/genji/types"
	"go.uber.org/atomic"
	"sync"
)

type ProfileStorage struct {
	closed atomic.Bool
	sync.Mutex
	db        *genji.DB
	metaCache map[meta.ProfileTarget]string
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
	return &ProfileStorage{
		db:        db,
		metaCache: make(map[meta.ProfileTarget]string),
	}, nil
}

var ErrStoreIsClosed = errors.New("storage is closed")

func (s *ProfileStorage) AddProfile(pt meta.ProfileTarget, ts int64, profile []byte) error {
	if s.isClose() {
		return ErrStoreIsClosed
	}
	tbName, err := s.prepareProfileTable(pt)
	if err != nil {
		return err
	}

	sql := fmt.Sprintf("INSERT INTO %v (ts, data) VALUES (?, ?)", tbName)
	s.Lock()
	defer s.Unlock()
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
	sqlBuf := bytes.NewBuffer(make([]byte, 0, 32))
	for _, pt := range targets {
		exist := s.isProfileTargetExist(pt)
		if !exist {
			result = append(result, meta.ProfileList{
				Target: pt,
			})
			continue
		}

		sqlBuf.Reset()
		sqlBuf.WriteString(fmt.Sprintf("SELECT ts FROM %v WHERE ts >= ? and ts <= ?", s.getProfileTableName(pt)))
		fmt.Println(sqlBuf.String(), args)
		res, err := s.db.Query(sqlBuf.String(), args...)
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

func (s *ProfileStorage) isProfileTargetExist(pt meta.ProfileTarget) bool {
	s.Lock()
	_, ok := s.metaCache[pt]
	s.Unlock()
	return ok
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

func (s *ProfileStorage) prepareProfileTable(pt meta.ProfileTarget) (string, error) {
	s.Lock()
	defer s.Unlock()
	tbName, ok := s.metaCache[pt]
	if ok {
		return tbName, nil
	}
	var err error
	tbName, err = s.createProfileTable(pt)
	if err != nil {
		return tbName, err
	}
	// update cache
	s.metaCache[pt] = tbName
	return tbName, nil
}

func (s *ProfileStorage) createProfileTable(pt meta.ProfileTarget) (string, error) {
	tbName := s.getProfileTableName(pt)
	sql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %v (ts INTEGER PRIMARY KEY, data BLOB)", tbName)
	err := s.db.Exec(sql)
	return tbName, err
}

func (s *ProfileStorage) getProfileTableName(pt meta.ProfileTarget) string {
	return fmt.Sprintf("`profile_%v_%v_%v`", pt.Tp, pt.Job, pt.Address)
}
