package store

import (
	"context"
	"fmt"
	"github.com/crazycs520/continuous-profile/meta"
	"github.com/dgraph-io/badger/v3"
	"github.com/dgraph-io/badger/v3/options"
	"github.com/genjidb/genji"
	"github.com/genjidb/genji/engine/badgerengine"
	"sync"
)

type ProfileStorage struct {
	sync.Mutex
	db        *genji.DB
	metaCache map[meta.ProfileTarget]string
}

func NewProfileStorage(storagePath string) (*ProfileStorage, error) {
	opts := badger.DefaultOptions(storagePath).
		WithCompression(options.ZSTD).
		WithZSTDCompressionLevel(3).
		WithBlockSize(8 * 1024 * 1024).
		WithValueThreshold(8 * 1024 * 1024)
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

func (s *ProfileStorage) AddProfile(pt meta.ProfileTarget, ts int64, profile []byte) error {
	tbName, err := s.prepareProfileTable(pt)
	if err != nil {
		return err
	}

	sql := fmt.Sprintf("INSERT INTO %v (ts, data) VALUES (?, ?)", tbName)
	s.Lock()
	defer s.Unlock()
	return s.db.Exec(sql, ts, profile)
}

func (s *ProfileStorage) Close() error {
	return s.db.Close()
}

func (s *ProfileStorage) prepareProfileTable(pt meta.ProfileTarget) (string, error) {
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
	s.Lock()
	defer s.Unlock()
	err := s.db.Exec(sql)
	return tbName, err
}

func (s *ProfileStorage) getProfileTableName(pt meta.ProfileTarget) string {
	return fmt.Sprintf("`profile_%v_%v_%v`", pt.Tp, pt.Job, pt.Address)
}
