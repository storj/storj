// +build !windows

package diskstore

import (
	"fmt"
	"syscall"
)

func isBusy(err error) bool {
	err = underlyingError(err)
	return err == syscall.EBUSY
}

func diskInfoFromPath(path string) (filesytemId string, amount int64, err error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return 0, -1, err
	}

	amount = int64(stat.Bavail) * int64(stat.Bsize)
	filesytemId = fmt.Sprintf("%08x%08x", stat.Fsid.Val[0], stat.Fsid.Val[1])

	return filesytemId, amount, nil
}
