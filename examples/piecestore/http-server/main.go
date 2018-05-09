package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"

	"storj.io/storj/pkg/piecestore"
)

var dataDir string

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
	if dataSizeStr == "" || dataSizeInt64 < 0 {
		dataSize = header.Size
	} else {
		dataSize = dataSizeInt64
	}

	var pstoreOffset int64
	pstoreOffsetStr := strings.Join(r.Form["offset"], "")
	pstoreOffsetInt64, _ := strconv.ParseInt(pstoreOffsetStr, 10, 64)
	if pstoreOffsetStr == "" && pstoreOffsetInt64 < 0 {
		pstoreOffset = 0
	} else {
		pstoreOffset = pstoreOffsetInt64
	}

	dataHash := strings.Join(r.Form["hash"], "")
	if dataHash == "" {
		dataHash = String(20)
	}

	defer file.Close()
	fmt.Printf("Uploading file (%s), Hash: (%s), Offset: (%v), Size: (%v)...\n", header.Filename, dataHash, pstoreOffset, dataSize)

	err = pstore.Store(dataHash, file, dataSize, pstoreOffset, dataDir)

	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		return
	}

	success := fmt.Sprintf("Successfully uploaded file!\nName: %s\nHash: %s\nSize: %v\n", header.Filename, dataHash, header.Size)
	fmt.Printf(success)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	message := fmt.Sprintf("%s\n<a href=\"/files/\">List files</a>", success)
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

	var pstoreOffset int64
	pstoreOffsetStr := strings.Join(r.Form["offset"], "")
	pstoreOffsetInt64, _ := strconv.ParseInt(pstoreOffsetStr, 10, 64)
	if pstoreOffsetStr == "" && pstoreOffsetInt64 <= 0 {
		pstoreOffset = 0
	} else {
		pstoreOffset = pstoreOffsetInt64
	}
	fmt.Printf("Downloading file (%s), Offset: (%v), Size: (%v)...\n", hash, pstoreOffset, length)

	err := pstore.Retrieve(hash, w, length, pstoreOffset, dataDir)

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
	port := "8080"

	if len(os.Args) > 1 {
		if matched, _ := regexp.MatchString(`^\d{2,6}$`, os.Args[1]); matched == true {
			port = os.Args[1]
		}
	}

	fmt.Printf("Starting server at port %s...\n", port)

	dataDir = path.Join("./piece-store-data/", port)

	router := httprouter.New()
	router.GET("/", Index)
	router.GET("/upload", ShowUploadForm)
	router.GET("/download", ShowDownloadForm)
	router.ServeFiles("/files/*filepath", http.Dir(dataDir))
	router.POST("/upload", UploadFile)
	router.POST("/download", DownloadFile)
	log.Fatal(http.ListenAndServe(":"+port, router))
}
