package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"

	"storj.io/storj/pkg/pstore"
)

const dataDir = "./piece-store-data"

func UploadFile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	// in your case file would be fileupload
	file, header, err := r.FormFile("uploadfile")
	if err != nil {
		fmt.Printf("Error: ", err.Error())
		return
	}

	// We have to do stupid conversions
	// Is there a better way to convert []string into ints?
	r.ParseForm()
	var dataSize int64
	dataSizeStr := strings.Join(r.Form["size"], "")
	dataSizeInt64, _ := strconv.ParseInt(dataSizeStr, 10, 64)
	if dataSizeStr == "" || dataSizeInt64 <= 0 {
		dataSize = header.Size
	} else {
		dataSize = dataSizeInt64
	}

	var dataOffset int64
	dataOffsetStr := strings.Join(r.Form["offset"], "")
	dataOffsetInt64, _ := strconv.ParseInt(dataOffsetStr, 10, 64)
	if dataOffsetStr == "" && dataOffsetInt64 <= 0 {
		dataOffset = 0
	} else {
		dataOffset = dataOffsetInt64
	}

	defer file.Close()
	fmt.Printf("Uploading file (%s), Offset: (%v), Size: (%v)...\n", header.Filename, dataOffset, dataSize)

	hash := String(20)
	err = pstore.Store(hash, file, dataSize, dataOffset, dataDir)

	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		return
	}

	fmt.Printf("Successfully uploaded file %s...\n", header.Filename)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	message := fmt.Sprintf("Successfully uploaded file!\nName: %s\nHash: %s\nSize: %v\n", header.Filename, hash, header.Size)
	message = fmt.Sprintf("%s\n<a href=\"/files/\">List files</a>", message)
	w.Write([]byte(message))
}

func DownloadFile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	r.ParseForm()

	hash := strings.Join(r.Form["hash"], "")

	var length int64
	lengthStr := strings.Join(r.Form["length"], "")
	length64, _ := strconv.ParseInt(lengthStr, 10, 64)
	if lengthStr == "" && length64 <= 0 {
		length = -1
	} else {
		length = length64
	}

	var dataOffset int64
	dataOffsetStr := strings.Join(r.Form["offset"], "")
	dataOffsetInt64, _ := strconv.ParseInt(dataOffsetStr, 10, 64)
	if dataOffsetStr == "" && dataOffsetInt64 <= 0 {
		dataOffset = 0
	} else {
		dataOffset = dataOffsetInt64
	}
	fmt.Printf("Downloading file (%s), Offset: (%v), Size: (%v)...\n", hash, dataOffset, length)

	err := pstore.Retrieve(hash, w, length, dataOffset, dataDir)

	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		w.Write([]byte(err.Error()))
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+hash)
	w.Header().Set("Content-Type", r.Header.Get("Content-Type"))

	fmt.Printf("Successfully downloaded file %s...\n", hash)
}

func Index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if err := renderByPath(w, "./server/templates/index.html"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func ShowUploadForm(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if err := renderByPath(w, "./server/templates/uploadform.html"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func ShowDownloadForm(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if err := renderByPath(w, "./server/templates/downloadform.html"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func renderByPath(w http.ResponseWriter, path string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	tmpl, err := template.ParseFiles(path)
	if err != nil {
		return err
	}

	tmpl.Execute(w, "")
	return nil
}

func main() {
	router := httprouter.New()
	router.GET("/", Index)
	router.GET("/upload", ShowUploadForm)
	router.GET("/download", ShowDownloadForm)
	router.ServeFiles("/files/*filepath", http.Dir(dataDir))
	router.POST("/upload", UploadFile)
	router.POST("/download", DownloadFile)
	log.Fatal(http.ListenAndServe(":8080", router))
}
