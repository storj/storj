package global

import "os"

type params struct {
	cwd string
}

var (
	Params *params
)

func init() {
	Params = newParams()
}

func newParams() (globalParams *params) {
	globalParams = new(params)

	dir, err := os.Getwd()
	if err == nil {
		globalParams.cwd = dir
	}

	return
}

func (p *params) GetCwd() string {
	return p.cwd
}
