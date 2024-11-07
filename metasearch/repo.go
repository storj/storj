package metasearch

// metadata search repo represent a collection of operations on metadata
type MetaSearchRepo interface {
	View(path string) (meta map[string]interface{}, err error)
	Query(page int, query, path string) (meta map[string]interface{}, err error)
	CreateUpdate(path, metadata string) (err error)
	Delete(path string) (err error)
}
