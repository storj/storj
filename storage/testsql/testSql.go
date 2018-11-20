package testsql

import (
	"fmt"
	"math/rand"
)

const (
	//Driver is the name of the SQL driver used in test dbs
	Driver = "sqlite3"
)

//Path returns the connection string for a new test database
func Path() string {
	return fmt.Sprintf("file:memdb%d?mode=memory&cache=shared", rand.Int63())
}
