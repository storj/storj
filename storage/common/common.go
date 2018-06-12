package storage

// DB interface allows more modular unit testing
// and makes it easier in the future to substitute
// db clients other than bolt
type DB interface {
  Put([]byte, []byte) error
  Get([]byte) ([]byte, error)
  List() ([][]byte, error)
  Delete([]byte) error
  Close() error
}

