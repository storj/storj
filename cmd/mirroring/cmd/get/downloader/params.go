package downloader

const (
	DEFAULT_DELIMITER = "/"
	DEFAULT_MAX_KEYS = 100
)

type Params struct {
	delimiter, prefix string
	marker string
	token string
	startAfter string
	rootPath, path string
	fetchOwner bool
	recursive bool
	maxKeys int
}

func NewDefaultParams() *Params {
	return &Params{
		delimiter: DEFAULT_DELIMITER,
		prefix: "",
		maxKeys: DEFAULT_MAX_KEYS,
	}
}

func (p *Params) SetPath(path string) {
	p.path = path
}

func (p *Params) SetRecursive(r bool) {
	p.recursive = r
}

func (p *Params) SetPrefix(prefix string) {
	p.prefix = prefix
}
