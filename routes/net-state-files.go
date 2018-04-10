package routes

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"

	"github.com/storj/storage/boltdb"
)

var (
	ErrReadReq = errors.New("error reading request body")
)

type File struct {
	DB *boltdb.Client
}

type Message struct {
	Value string `json:"value"`
}

func NewFile(db *boltdb.Client) *File {
	return &File{DB: db}
}

func (f *File) Put(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	givenPath := ps.ByName("path")

	var msg Message
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&msg)
	if err != nil {
		log.Printf("err decoding response: %v", err)
		return
	}

	file := boltdb.File{
		Path:  givenPath,
		Value: msg.Value,
	}

	if err := f.DB.Put(file); err != nil {
		fmt.Println(err)
	}

	fmt.Fprintf(w, "PUT to %s\n", givenPath)
}

func (f *File) Get(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	fileKey := ps.ByName("path")

	fileInfo, err := f.DB.Get([]byte(fileKey))
	if err != nil {
		fmt.Println(err)
	}

	fmt.Fprintf(w, "value: %s", fileInfo)
}

func (f *File) List(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	fileKeys, err := f.DB.List([]byte("files"))
	if err != nil {
		fmt.Println(err)
	}

	fmt.Fprintf(w, "All stored file paths: %s", fileKeys)
}

func (f *File) Delete(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	fileKey := ps.ByName("path")
	f.DB.Delete([]byte(fileKey))

	fmt.Fprintf(w, "Deleted file key: %s", fileKey)
}
