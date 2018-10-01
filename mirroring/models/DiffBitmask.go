package models

type DiffBitmask uint8

const (
	IN_MAIN DiffBitmask = 1 << iota
	IN_MIRROR
	NAME
)

//HasFlag is
func (f DiffBitmask) HasFlag(flag DiffBitmask) bool {
	return f&flag != 0
}

//AddFlag is
func (f *DiffBitmask) AddFlag(flag DiffBitmask) {
	*f |= flag
}

//ClearFlag is
func (f *DiffBitmask) ClearFlag(flag DiffBitmask) {
	*f &= ^flag
}

//ToggleFlag is
func (f *DiffBitmask) ToggleFlag(flag DiffBitmask) {
	*f ^= flag
}
