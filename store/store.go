package store

type Storage interface {
	Get(k []byte) ([]byte, error)
	Set(k, v []byte) error
	Close() error
}
