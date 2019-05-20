package marketing

// DB contains access to all marketing related databases
type DB interface {
	Offers() Offers
}
