package badger

import (
	"github.com/dgraph-io/badger/v3"
	"github.com/dgraph-io/badger/v3/options"
)

type DB struct {
	*badger.DB
}

func NewDB(storagePath string) (*DB, error) {
	dbOption := badger.DefaultOptions(storagePath).
		WithCompression(options.ZSTD).
		WithZSTDCompressionLevel(10).
		WithBlockSize(4 * 1024 * 1024)
	db, err := badger.Open(dbOption)
	if err != nil {
		return nil, err
	}
	return &DB{DB: db}, nil
}

func (db *DB) Get(k []byte) ([]byte, error) {
	var data []byte
	err := db.DB.View(func(txn *badger.Txn) error {
		item, err := txn.Get(k)
		if err != nil {
			return err
		}
		err = item.Value(func(val []byte) error {
			data = make([]byte, len(val))
			copy(data, val)
			return nil
		})
		return err
	})
	return data, err
}

func (db *DB) Set(k, v []byte) error {
	return db.DB.Update(func(txn *badger.Txn) error {
		return txn.Set(k, v)
	})
}

func (db *DB) Close() error {
	return db.DB.Close()
}
