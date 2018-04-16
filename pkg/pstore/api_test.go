package pstore // import "storj.io/storj/pkg/pstore"

import (
    "testing"
    "os"
    "path"
    "bufio"
    "fmt"
)


var testFile = path.Join(os.TempDir(), "test.txt")

func createTestFile() (error) {
  file, createFileErr := os.Create(testFile)

  if createFileErr != nil {
    return createFileErr
  }

  b := []byte{'b', 'u', 't', 't', 's'}
  _, writeToFileErr := file.Write(b)

  if writeToFileErr != nil {
    return writeToFileErr
  }

  return nil
}

func deleteTestFile() (error) {
  err := os.Remove(testFile)
  if err != nil {
    return err
  }

  return nil
}

func TestStore(t *testing.T) {
  createTestFile()
  defer deleteTestFile()
  file, openTestFileErr := os.Open(testFile)
  if openTestFileErr != nil {
    t.Errorf("Could not open test file")
    return
  }

  defer file.Close()

  reader := bufio.NewReader(file)

  hash := "0123456789ABCDEFGHIJ"
  Store(hash, reader, os.TempDir())

  folder1 := string(hash[0:2])
  folder2 := string(hash[2:4])
  fileName := string(hash[4:])

  createdFilePath := path.Join(os.TempDir(), folder1, folder2, fileName)
  defer os.RemoveAll(path.Join(os.TempDir(), folder1))
  _, lStatErr := os.Lstat(createdFilePath)
  if lStatErr != nil {
    t.Errorf("No file was created from Store(): %s", lStatErr.Error())
    return
  }

  createdFile, openCreatedError := os.Open(createdFilePath)
  if openCreatedError != nil {
    t.Errorf("Error: %s opening created file %s", openCreatedError.Error() ,createdFilePath)
  }
  defer createdFile.Close()

  buffer := make([]byte, 5)
  _, _ = createdFile.Read(buffer)


  if string(buffer) != "butts" {
    t.Errorf("Expected data butts does not equal Actual data %s", string(buffer))
  }
}

func TestRetrieve(t *testing.T) {
  createTestFile()
  defer deleteTestFile()
  file, openTestFileErr := os.Open(testFile)
  if openTestFileErr != nil {
    t.Errorf("Could not open test file")
    return
  }

  defer file.Close()

  reader := bufio.NewReader(file)

  hash := "0123456789ABCDEFGHIJ"
  Store(hash, reader, os.TempDir())

  // Create file for retrieving data into
  retrievalFilePath := path.Join(os.TempDir(), "retrieved.txt")
  retrievalFile, retrievalFileError := os.OpenFile(retrievalFilePath, os.O_RDWR|os.O_CREATE, 0777)
  if retrievalFileError != nil {
    t.Errorf("Error creating file: %s", retrievalFileError.Error())
    return
  }
  defer retrievalFile.Close()

  writer := bufio.NewWriter(retrievalFile)

  retrieveErr := Retrieve(hash, writer, os.TempDir())

  if retrieveErr != nil {
    t.Errorf("Retrieve Error: %s", retrieveErr.Error())
  }

  buffer := make([]byte, 5)

  retrievalFile.Seek(0,0)
  _, _ = retrievalFile.Read(buffer)

  fmt.Printf("Retrieved data: %s", string(buffer))

  if string(buffer) != "butts" {
    t.Errorf("Expected data butts does not equal Actual data %s", string(buffer))
  }
  // Verify that the contents of the retrieve match what was stored
  // delete the folders and file
}

func TestDelete(t *testing.T) {
  // Run Store()
  // Verify files exist
  // Run Delete()
  // Verify that the files are no longer existing
}

func TestGetStoreInfo(t *testing.T) {
    GetStoreInfo("/tmp/")
}
