package endpoints

import (
	"os"
)

var (
	fname    string = "graphql_schema.txt"
	fcontrol *testfile
	fuut     *testfile
	req      []byte
	treq     []byte
	saturl   string = "https://satellite.qa.storj.io/api/v0/graphql"
	uutname  string = "uut.txt"
)

func Main() {
	//delete the graphql_schema.txt if the endpoints were modified.
	//a new one with updated data will automaticall be created.
	req = introspect(saturl) //call introspect from handler.go

	isfile := checkfile(fname) //check if file exists if yes then open else create one
	if !isfile {               //this is control file
		fcontrol = newtestfile(fname, req)
	}
	fcontrol = openfile(fname)

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
		os.Exit(1)            //
	}
	deletefile(fuut.name)
	os.Exit(0)
}
