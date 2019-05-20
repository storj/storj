package console_test

import (
	"fmt"
	"testing"
)

func Test(t *testing.T) {
	f := func() (err int) {
		defer func() {
			if err != 0 {
				err = 4
			}
		}()

		return 1
	}

	f2 := func() *int {
		var err int
		defer func() {
			if err != 0 {
				err = 4
			}
		}()

		err = 1
		return &err
	}

	f3 := func() int {
		var err int
		defer func() {
			if err != 0 {
				err = 4
			}
		}()

		err = 1
		return err
	}

	v1 := f()
	v2 := *f2()
	v3 := f3()

	fmt.Println(v1)
	fmt.Println(v2)
	fmt.Println(v3)
}
