package conndebug

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io/ioutil"
	"os"
	"runtime/debug"
	"sort"
	"sync"
	"time"

	"github.com/lib/pq"
)

// Driver is a driver debugger
type Driver struct {
	driver driver.Driver

	mu    sync.Mutex
	conns map[*Conn]bool
}

// ConnEverything ignore
type ConnEverything interface {
	driver.Conn
	driver.ConnBeginTx
	driver.Execer
	driver.ExecerContext
	driver.Pinger
	driver.Queryer
	driver.QueryerContext
}

// Conn ignore
type Conn struct {
	ConnEverything

	driver *Driver

	trace  []byte
	opened time.Time
	closed time.Time
}

// Open ignore
func (driver *Driver) Open(name string) (driver.Conn, error) {
	underlying, err := driver.driver.Open(name)
	if err != nil {
		return underlying, err
	}
	conn := &Conn{}
	conn.ConnEverything = underlying.(ConnEverything)
	conn.driver = driver
	conn.opened = time.Now()
	conn.trace = debug.Stack()

	driver.RegisterConn(conn)
	return conn, err
}

// RegisterConn ignore
func (driver *Driver) RegisterConn(conn *Conn) {
	driver.mu.Lock()
	defer driver.mu.Unlock()

	driver.conns[conn] = true
}

// UnregisterConn ignore
func (driver *Driver) UnregisterConn(conn *Conn) {
	driver.mu.Lock()
	defer driver.mu.Unlock()

	conn.closed = time.Now()
	driver.conns[conn] = false
}

// Close ignore
func (conn *Conn) Close() error {
	conn.driver.UnregisterConn(conn)
	return conn.ConnEverything.Close()
}

// WriteTo ignore
func (driver *Driver) WriteTo(w *bytes.Buffer) {
	type Info struct {
		Trace  []byte
		Opened time.Time
		Closed time.Time
	}

	infos := make([]Info, 0, 1000)

	driver.mu.Lock()
	for conn := range driver.conns {
		infos = append(infos, Info{
			Trace:  conn.trace,
			Opened: conn.opened,
			Closed: conn.closed,
		})
	}
	driver.mu.Unlock()

	total := len(infos)
	closed := 0
	for _, info := range infos {
		if !info.Closed.IsZero() {
			closed++
		}
	}

	fmt.Fprintf(w, "total  = %v\n", total)
	fmt.Fprintf(w, "alive  = %v\n", total-closed)
	fmt.Fprintf(w, "closed = %v\n\n\n", closed)

	sort.Slice(infos, func(i, k int) bool {
		if infos[i].Closed.Equal(infos[k].Closed) {
			return infos[i].Opened.Before(infos[k].Opened)
		}
		return infos[i].Closed.Before(infos[k].Closed)
	})

	now := time.Now()
	for _, info := range infos {
		status := "alive"
		aliveness := now.Sub(info.Opened)
		if !info.Closed.IsZero() {
			aliveness = info.Closed.Sub(info.Opened)
			status = "closed"
			closed++
		}
		fmt.Fprintf(w, "%v: %s, life=%v (closed %v):\n%v\n\n", info.Opened, status, aliveness, info.Closed, string(info.Trace))
	}
}

func init() {
	driver := &Driver{
		driver: &pq.Driver{},
		conns:  map[*Conn]bool{},
	}
	go func() {
		file, err := ioutil.TempFile("", "postgres-conn-*.log")
		if err != nil {
			panic(err)
		}
		_ = file.Close()
		fmt.Fprintf(os.Stderr, "Writing postgres connection debug to %q\n", file.Name())

		ticker := time.NewTicker(15 * time.Second)
		for range ticker.C {
			var buffer bytes.Buffer
			driver.WriteTo(&buffer)

			err := ioutil.WriteFile(file.Name(), buffer.Bytes(), 0755)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to write postgres-conn file: %v\n", err)
			}
		}
	}()
	sql.Register("postgres-debug", driver)
}
