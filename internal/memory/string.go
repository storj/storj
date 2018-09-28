package memory

// ToString converts number of bytes to appropriately sized string
func ToString(bytes int64) string {
	size := Size{}
	size.Bytes = bytes
	return size.String()
}

// ParseString converts string to number of bytes
func ParseString(s string) (int64, error) {
	size := Size{}
	err := size.Set(s)
	return size.Bytes, err
}
