# FilePiece

Concurrently read and write from files

## Installation
```BASH
go get storj.io/storj/pkg/filepiece
```

## Usage
```Golang
import "storj.io/storj/pkg/filepiece"
```

### Chunk struct
```Golang
type Chunk struct {
	file       *os.File
	offset     int64
	length     int64
	currentPos int64
}
```
* Chunk.file - os.File being read from
* Chunk.offset - starting position for reading/writing data
* Chunk.length - length of data to be read/written
* Chunk.currentPos - Keeps track to know where to write to or read from next

### NewChunk
Create a chunk from a file
```Golang
func NewChunk(file *os.File, offset int64, length int64) (*Chunk, error)
```

### Read
Concurrently read from a file
```Golang
func (f *Chunk) Read(b []byte) (n int, err error)
```
```Golang
func (f *Chunk) ReadAt(p []byte, off int64) (n int, err error)
```

### Write
Concurrently write to a file
```Golang
func (f *Chunk) Write(b []byte) (n int, err error)
```
```Golang
func (f *Chunk) WriteAt(p []byte, off int64) (n int, err error)
```

### Other
Get the size of the Chunk
```Golang
func (f *Chunk) Size() int64
```

Close the Chunk File
```Golang
func (f *Chunk) Close() error
```

Seek to certain position of Chunk
```Golang
func (f *Chunk) Seek(offset int64, whence int) (int64, error)
```
