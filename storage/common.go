package storage

type Key []byte
type Value []byte
type Keys []Key

// KeyValueStore interface allows more modular unit testing
// and makes it easier in the future to substitute
// db clients other than bolt
type KeyValueStore interface {
	Put(Key, Value) error
	Get(Key) (Value, error)
	List() (Keys, error)
	Delete(Key) error
	Close() error
}

func (v *Value) MarshalBinary() (_ []byte, _ error) {
	return *v, nil
}

func (k *Key) MarshalBinary() (_ []byte, _ error) {
	return *k, nil
}

func (k *Key) String() string {
	return string(*k)
}

func (k *Keys) ByteSlices() [][]byte {
	result := make([][]byte, len(*k))

	for _k, v := range *k {
		result[_k] = []byte(v)
	}

	return result
}
