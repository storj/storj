package pstore // import "storj.io/storj/pkg/pstore"

import (
	"fmt"
  "bufio"
  "io"
	"os"
	"path"
)

type argError struct {
	arg string
	msg string
}

func (e *argError) Error() string {
  return fmt.Sprintf("HashError (%s): %s", string(e.arg), e.msg)
}

type fsError struct {
  path string
  msg string
}

func (e *fsError) Error() string {
  return fmt.Sprintf("FSError (%s): %s", e.path, e.msg)
}

func Store(hash string, r *bufio.Reader, dir string) (error) {
  fmt.Println("Storing...")
  if len(hash) < 20 {
    return &argError{hash, "Hash is too short"}
  }
	if dir == "" {
    return &argError{dir, "No path provided"}
  }

	// Folder structure
  folder1 := string(hash[0:2])
  folder2 := string(hash[2:4])
  fileName := string(hash[4:])

	// Create directory path string
	dirpath := path.Join(dir, folder1, folder2)

	// Create directory path on file system
	mkDirerr := os.MkdirAll(dirpath, 0700)
	if mkDirerr != nil {
		return mkDirerr
	}

	// Create File Path string
	filepath := path.Join(dirpath, fileName)

	// Create File on file system
	file, openErr := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE, 0755)
	if openErr != nil {
		return openErr
	}

	// Close when finished
	defer file.Close()

	// Buffer for reading data
  buffer := make([]byte, 4096)
	for {
		// Read data from read stream into buffer
		n, err := r.Read(buffer)
		if err == io.EOF {
			break
		}

		// Write the buffer to the file we opened earlier
		_, err = file.Write(buffer[:n])
	}

  return nil
}

func Retrieve(hash string, w *bufio.Writer, dir string) (error) {
  fmt.Println("Retrieving...")
	if len(hash) < 20 {
		return &argError{hash, "Hash too short"}
	}
	if dir == "" {
    return &argError{dir, "No path provided"}
  }

	folder1 := string(hash[0:2])
  folder2 := string(hash[2:4])
  fileName := string(hash[4:])

	filePath := path.Join(dir, folder1, folder2, fileName)

	file, openErr := os.OpenFile(filePath, os.O_RDONLY, 0755)
	if openErr != nil {
		return openErr
	}
	// Close when finished
	defer file.Close()

	file.Seek(0,0)
	buffer := make([]byte, 4096)
	for {
		// Read data from read stream into buffer
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}

		fmt.Println("read buffer: ", string(buffer))
		fmt.Println("read bytes: ", n)
		// Write to buffer to the file we opened earlier
		writtenbytes, writeError := w.Write(buffer[:n])
		fmt.Println("written bytes: ",writtenbytes)
		if writeError != nil {
			fmt.Println("Write Error:", writeError)
		}

	}

	w.Flush()

  return nil
}

func Delete(hash string, dir string) (error) {
  fmt.Println("Deleting...")
	if len(hash) < 20 {
		return &argError{hash, "Hash too short"}
	}
	if dir == "" {
    return &argError{dir, "No path provided"}
  }

	folder1 := string(hash[0:2])
  folder2 := string(hash[2:4])
  fileName := string(hash[4:])

	err := os.Remove(path.Join(dir, folder1, folder2, fileName))
	if err != nil {
		return err
	}

  return nil
}

func GetStoreInfo(dir string) {
  fmt.Println("Getting store info")
}
