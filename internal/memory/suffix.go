package memory

// Unit represents a memory unit with a suffix and scale
type Unit struct {
	Suffix string
	Scale  Size
}

// Different byte-size suffixes
const (
	TB = 1 << 40
	GB = 1 << 30
	MB = 1 << 20
	KB = 1 << 10
	B  = 1
)

// List of units
var Units = []Unit{
	{"T", TB},
	{"G", GB},
	{"M", MB},
	{"K", KB},
	{"B", B},
	{"", 0},
}
