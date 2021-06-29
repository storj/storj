package endpoints

//delete the graphql_schema.txt if the endpoints were modified.
//a new one with updated data will automatically be created.
import (
	"fmt"
	"os"
	"path/filepath"
)

var (
	fname    string = "web/satellite/tests/graphql/endpoints/graphql_schema.txt"
	fcontrol *testfile
	fuut     *testfile
	req      []byte
	treq     []byte
	satenv   string
	saturl   string = "/api/v0/graphql"
	uutname  string = "web/satellite/tests/graphql/endpoints/uut.txt"
	exitcode        = 0
)

func Endpoints() int {
	//sets the path up for the environment.
	fname = filepath.FromSlash(fname)

	//build the satellite url from the environment variable.
	satenv = os.Getenv("SATELLITE_0_ADDR")
	saturl = "http://" + satenv + saturl

	isfile := checkfile(fname) //check if file exists if yes then open else create one
	if !isfile {               //this is control file
		req = introspect(saturl) //call introspect from handler.go
		fcontrol = newtestfile(fname, req)
		fmt.Println(fname, "not is file - new test file")
	}
	fcontrol = openfile(fname)
	fmt.Println(fname, "file either existed or was just recently created - open test file")
	treq = introspect(saturl) //call introspect

	uut := checkfile(uutname) //check if file exists if yes then open else create one
	if !uut {
		fuut = newtestfile(uutname, treq)
	}
	fuut = openfile(uutname)

	compcontrol := len(string(fcontrol.contents)) //compute the length of the control file - the introspection query sends back
	compuut := len(string(fuut.contents))         //data differently most of the time so I cannot compair the byte[]
	if compcontrol != compuut {                   //if they are not equal then graphql endpoints were modified
		//this test will have a false positive if an enpoint is removed and one of the same length is added
		deletefile(fuut.name) //delete the unit under test file
		exitcode = 6
	}
	deletefile(fuut.name)
	return exitcode
}
