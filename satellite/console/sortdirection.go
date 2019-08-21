package console

// OrderDirection is used for members in specific order direction
type OrderDirection uint8

const (
	// Ascending indicates that we should order ascending
	Ascending OrderDirection = 1
	// Descending indicates that we should order descending
	Descending OrderDirection = 2
)
