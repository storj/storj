// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package fpiece

import (
	"io/ioutil"
	"log"
	"os"
	"testing"
)

var readTests = []struct {
	in     string
	offset int64
	len    int64
	out    string
}{
	{"butts", 0, 5, "butts"},
	{"butts", 0, 2, "bu"},
	{"butts", 3, 2, "ts"},
	{"butts", 0, 10, "butts"},
	{"butts", 1, 1100, "utts"},
}

var readAtTests = []struct {
	in     string
	offset int64
	out    string
}{
	{"butts", 0, "butts"},
	{"butts", 1, "utts"},
	{"butts", 2, "tts"},
	{"butts", 100, ""},
	{"butts", -1, ""},
}

var writeTests = []struct {
	in     string
	offset int64
	len    int64
	out    string
}{
	{"butts", 0, 5, "butts"},
	{"butts", 0, 2, "bu"},
	{"butts", 3, 2, "\x00\x00\x00bu"},
	{"butts", 0, 10, "butts"},
	{"butts", 1, 1100, "\x00butts"},
}

var writeAtTests = []struct {
	in     string
	offset int64
	out    string
}{
	{"butts", 0, "butts"},
	{"butts", 1, "\x00butt"},
	{"butts", 3, "\x00\x00\x00bu"},
	{"butts", 1000, ""},
	{"butts", -11, ""},
}

func TestRead(t *testing.T) {

	for _, tt := range readTests {
		t.Run("Reads data properly", func(t *testing.T) {

			tmpfilePtr, err := ioutil.TempFile("", "read_test")
			if err != nil {
				log.Fatal(err)
			}

			defer os.Remove(tmpfilePtr.Name()) // clean up

			if _, err := tmpfilePtr.Write([]byte(tt.in)); err != nil {
				log.Fatal(err)
			}

			chunk, err := NewChunk(tmpfilePtr, tt.offset, tt.len)
			if err != nil {
				log.Fatal(err)
			}

			buffer := make([]byte, 100)
			n, _ := chunk.Read(buffer)

			if err := tmpfilePtr.Close(); err != nil {
				log.Fatal(err)
			}

			if string(buffer[:n]) != tt.out {
				t.Errorf("got %q, want %q", string(buffer[:n]), tt.out)
			}

		})
	}

}

func TestReadAt(t *testing.T) {

	for _, tt := range readAtTests {
		t.Run("Reads data properly using ReadAt", func(t *testing.T) {

			tmpfilePtr, err := ioutil.TempFile("", "readAt_test")
			if err != nil {
				log.Fatal(err)
			}

			defer os.Remove(tmpfilePtr.Name()) // clean up

			if _, err := tmpfilePtr.Write([]byte(tt.in)); err != nil {
				log.Fatal(err)
			}

			chunk, err := NewChunk(tmpfilePtr, 0, int64(len(tt.in)))
			if err != nil {
				log.Fatal(err)
			}

			buffer := make([]byte, 100)
			n, _ := chunk.ReadAt(buffer, tt.offset)

			if err := tmpfilePtr.Close(); err != nil {
				log.Fatal(err)
			}

			if string(buffer[:n]) != tt.out {
				t.Errorf("got %q, want %q", string(buffer[:n]), tt.out)
			}

		})
	}

}

func TestWrite(t *testing.T) {

	for _, tt := range writeTests {
		t.Run("Writes data properly", func(t *testing.T) {

			tmpfilePtr, err := ioutil.TempFile("", "write_test")
			if err != nil {
				log.Fatal(err)
			}

			defer os.Remove(tmpfilePtr.Name()) // clean up

			chunk, err := NewChunk(tmpfilePtr, tt.offset, tt.len)
			if err != nil {
				log.Fatal(err)
			}

			chunk.Write([]byte(tt.in))

			buffer := make([]byte, 100)
			n, err := tmpfilePtr.Read(buffer)

			if err := tmpfilePtr.Close(); err != nil {
				log.Fatal(err)
			}

			if string(buffer[:n]) != tt.out {
				t.Errorf("got %q, want %q", string(buffer[:n]), tt.out)
			}

		})
	}

}

func TestWriteAt(t *testing.T) {

	for _, tt := range writeAtTests {
		t.Run("Writes data properly", func(t *testing.T) {

			tmpfilePtr, err := ioutil.TempFile("", "writeAt_test")
			if err != nil {
				log.Fatal(err)
			}

			defer os.Remove(tmpfilePtr.Name()) // clean up

			chunk, err := NewChunk(tmpfilePtr, 0, int64(len(tt.in)))
			if err != nil {
				log.Fatal(err)
			}

			chunk.WriteAt([]byte(tt.in), tt.offset)

			buffer := make([]byte, 100)
			n, err := tmpfilePtr.Read(buffer)

			if err := tmpfilePtr.Close(); err != nil {
				log.Fatal(err)
			}

			if string(buffer[:n]) != tt.out {
				t.Errorf("got %q, want %q", string(buffer[:n]), tt.out)
			}

		})
	}

}
