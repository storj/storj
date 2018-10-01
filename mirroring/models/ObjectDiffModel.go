package models

const (
	SIZE         DiffBitmask = 1 << 4
	CONTENT_TYPE DiffBitmask = 1 << 5
	IS_DIR       DiffBitmask = 1 << 6
)

//ObjectDiffModel is
type ObjectDiffModel struct {
	Name string
	Diff DiffBitmask
}
