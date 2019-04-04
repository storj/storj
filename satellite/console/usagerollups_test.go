package console

import (
	"fmt"
	"testing"
	"time"
)

func TestUsageRollups(t *testing.T) {
	fmt.Println(time.Now().Format(time.RFC3339))
	// 2018-04-02T18:25:11+03:00
	// 2020-04-05T18:25:11+03:00
	// 2019-04-03 17:16:26.822857431+00:00
}
