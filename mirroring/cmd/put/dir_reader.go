package put

import (
	"os"
	"io/ioutil"
)

type Dir interface {
	os.FileInfo
	Path() string
	Files() []os.FileInfo
	Dirs() []os.FileInfo
}

type dir struct {
	lpath string
	os.FileInfo
	files []os.FileInfo
	dirs []os.FileInfo
}

func (d *dir) Path() string {
	return d.lpath
}

func (d *dir) Files() []os.FileInfo {
	return d.files
}

func (d *dir) Dirs() []os.FileInfo {
	return d.dirs
}

type DirReader interface {
	ReadDir(lpath string) (Dir, error)
}

type dirReader struct {
}

func (d *dirReader) ReadDir(lpath string) (Dir, error) {
	info, err := os.Stat(lpath)
	if err != nil {
		return nil, err
	}

	items, err := ioutil.ReadDir(lpath)
	if err != nil {
		return nil, err
	}

	var files []os.FileInfo
	var dirs []os.FileInfo

	for i := range items {
		item := items[i]
		if item.IsDir() {
			dirs = append(dirs, item)
			continue
		}

		files = append(files, item)
	}

	return &dir{lpath,info, files, dirs}, nil
}
