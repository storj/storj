// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package redis

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/go-redis/redis"
	"github.com/stretchr/testify/assert"
	"storj.io/storj/internal/test"
	"storj.io/storj/storage"
)

type RedisClientTest struct {
	*testing.T
	c storage.KeyValueStore
}

func NewRedisClientTest(t *testing.T) *RedisClientTest {
	kv := make(test.KvStore)
	c := test.NewMockKeyValueStore(kv)
	return &RedisClientTest{
		T: t,
		c: c,
	}
}

func (rt *RedisClientTest) Close() {
	rt.c.Close()
}

func (rt *RedisClientTest) HandleErr(err error, msg string) {
	rt.Error(msg)
	if err != nil {
		panic(err)
	}
	panic(msg)
}

func TestListWithoutStartKey(t *testing.T) {
	done := test.EnsureRedis(t)
	defer done()

	rt := NewRedisClientTest(t)
	defer rt.Close()

	if err := rt.c.Put(storage.Key([]byte("path/1")), []byte("pointer1")); err != nil {
		rt.HandleErr(err, "Failed to put")
	}
	if err := rt.c.Put(storage.Key([]byte("path/2")), []byte("pointer2")); err != nil {
		rt.HandleErr(err, "Failed to put")
	}
	if err := rt.c.Put(storage.Key([]byte("path/3")), []byte("pointer3")); err != nil {
		rt.HandleErr(err, "Failed to put")
	}

	_, err := rt.c.List(nil, storage.Limit(3))
	if err != nil {
		rt.HandleErr(err, "Failed to list")
	}
}

func TestListWithStartKey(t *testing.T) {
	done := test.EnsureRedis(t)
	defer done()

	rt := NewRedisClientTest(t)
	defer rt.Close()

	if err := rt.c.Put(storage.Key([]byte("path/1")), []byte("pointer1")); err != nil {
		rt.HandleErr(err, "Failed to put")
	}
	if err := rt.c.Put(storage.Key([]byte("path/2")), []byte("pointer2")); err != nil {
		rt.HandleErr(err, "Failed to put")
	}
	if err := rt.c.Put(storage.Key([]byte("path/3")), []byte("pointer3")); err != nil {
		rt.HandleErr(err, "Failed to put")
	}
	if err := rt.c.Put(storage.Key([]byte("path/4")), []byte("pointer4")); err != nil {
		rt.HandleErr(err, "Failed to put")
	}
	if err := rt.c.Put(storage.Key([]byte("path/5")), []byte("pointer5")); err != nil {
		rt.HandleErr(err, "Failed to put")
	}

	_, err := rt.c.List([]byte("path/2"), storage.Limit(2))
	if err != nil {
		rt.HandleErr(err, "Failed to list")
	}
}

var (
	validConnection = "127.0.0.1:6379"
	unexistingKey   = "unexistingKey"
	validKey        = "validKey"
	validValue      = "validValue"
	keysList        = []string{"key1", "key2", "key3"}
	dbTest          = 1
)

type TestFunc func(t *testing.T, st storage.KeyValueStore)

func testWithRedis(t *testing.T, testFunc TestFunc) {
	st, err := NewClient(validConnection, "", dbTest)
	assert.NoError(t, err)
	assert.NotNil(t, st)

	defer func() {
		st.Close()
	}()

	testFunc(t, st)
}

func testWithInvalidConnection(t *testing.T, testFunc TestFunc) {
	st := &Client{
		db: redis.NewClient(&redis.Options{
			Addr:     "",
			Password: "",
			DB:       dbTest,
		}),
		TTL: defaultNodeExpiration,
	}

	testFunc(t, st)
}

func TestCloseRedis(t *testing.T) {
	testWithRedis(t, func(t *testing.T, st storage.KeyValueStore) {
		err := st.Close()
		assert.NoError(t, err)
	})
}

func TestNewClient(t *testing.T) {
	done := test.EnsureRedis(t)
	defer done()

	cases := []struct {
		testName, address string
		testFunc          func(storage.KeyValueStore, error)
	}{
		{
			"NotValidConnection",
			"",
			func(st storage.KeyValueStore, err error) {
				assert.Error(t, err)
				assert.Nil(t, st)
			},
		},
		{
			"ValidConnection",
			validConnection,
			func(st storage.KeyValueStore, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, st)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			storage, err := NewClient(c.address, "", dbTest)

			defer func() {
				if err == nil {
					storage.Close()
				}
			}()

			c.testFunc(storage, err)
		})
	}
}

func TestCrudValidConnection(t *testing.T) {
	cases := []struct {
		testName string
		testFunc TestFunc
	}{
		{
			"GetUnexistingKey",
			func(t *testing.T, st storage.KeyValueStore) {
				err := st.Delete(storage.Key(unexistingKey))
				assert.NoError(t, err)

				value, err := st.Get(storage.Key(unexistingKey))
				assert.True(t, storage.ErrKeyNotFound.Has(err))
				assert.Nil(t, value)
			},
		},
		{
			"GetValidKey",
			func(t *testing.T, st storage.KeyValueStore) {
				key := storage.Key(validKey)
				orgValue := storage.Value(validValue)

				err := st.Put(key, orgValue)
				assert.NoError(t, err)

				value, err := st.Get(key)
				assert.NoError(t, err)
				assert.Equal(t, orgValue, value)

				err = st.Delete(key)
				assert.NoError(t, err)
			},
		},
		{
			"UpdateKey",
			func(t *testing.T, st storage.KeyValueStore) {
				key := storage.Key(validKey)
				orgValue := storage.Value(validValue)

				err := st.Put(key, orgValue)
				assert.NoError(t, err)
				err = st.Put(key, orgValue)
				assert.NoError(t, err)

				err = st.Delete(key)
				assert.NoError(t, err)
			},
		},
		{
			"GetKeysList",
			func(t *testing.T, st storage.KeyValueStore) {
				orgValue := storage.Value(validValue)

				list := storage.Keys{}
				for _, key := range keysList {
					list = append(list, storage.Key(key))
				}

				for _, key := range list {
					err := st.Put(key, orgValue)
					assert.NoError(t, err)
				}

				//Temporary fix
				_, err := st.List(list[0], storage.Limit(len(keysList)))

				assert.NoError(t, err)
				// assert.ElementsMatch(t, list, keys)
				// assert.Equal(t, len(list), len(keys))

				for _, key := range list {
					err := st.Delete(key)
					assert.NoError(t, err)
				}
			},
		},
		{
			"GetKeysListWithFirstArgNil",
			func(t *testing.T, st storage.KeyValueStore) {
				keys, err := st.List(nil, storage.Limit(len(keysList)))
				assert.NoError(t, err)
				assert.Equal(t, len(keys), 0)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			testWithRedis(t, c.testFunc)
		})
	}
}

func TestCrudInvalidConnection(t *testing.T) {
	done := test.EnsureRedis(t)
	defer done()

	cases := []struct {
		testName string
		testFunc TestFunc
	}{
		{
			"Get",
			func(t *testing.T, st storage.KeyValueStore) {
				_, err := st.Get(storage.Key(validKey))
				assert.Error(t, err)
			},
		},
		{
			"Put",
			func(t *testing.T, st storage.KeyValueStore) {
				err := st.Put(storage.Key(validKey), storage.Value(validValue))
				assert.Error(t, err)
			},
		},
		{
			"Delete",
			func(t *testing.T, st storage.KeyValueStore) {
				err := st.Delete(storage.Key(validKey))
				assert.Error(t, err)
			},
		},
		{
			"ListArgValid",
			func(t *testing.T, st storage.KeyValueStore) {
				keys, err := st.List(storage.Key(validKey), 1)
				assert.Error(t, err)
				assert.Nil(t, keys)
			},
		},
		{
			"ListArgInvalid",
			func(t *testing.T, st storage.KeyValueStore) {
				keys, err := st.List(nil, 1)
				assert.Error(t, err)
				assert.Nil(t, keys)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			testWithInvalidConnection(t, c.testFunc)
		})
	}
}

func TestMain(m *testing.M) {
	cmd := exec.Command("redis-server")

	err := cmd.Start()

	if err != nil {
		fmt.Println(err)
		return
	}
	//waiting for "redis-server command" to start
	time.Sleep(time.Second)

	retCode := m.Run()

	err = cmd.Process.Kill()
	if err != nil {
		fmt.Print(err)
	}

	os.Exit(retCode)
}
