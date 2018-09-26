// +build !windows

package diskstore

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func isBusy(err error) bool {
	err = underlyingError(err)
	return err == unix.EBUSY
}

func diskInfoFromPath(path string) (filesytemId string, amount int64, err error) {
	var stat unix.Statfs_t
	err = unix.Statfs(path, &stat)
	if err != nil {
		return "", -1, err
	}

	amount = int64(stat.Bavail) * int64(stat.Bsize)
	filesytemId = fmt.Sprintf("%08x%08x", stat.Fsid.Val[0], stat.Fsid.Val[1])

	return filesytemId, amount, nil
}
