package writers

// Writer writes blobs with name to a given path
type Writer interface {
	Write(name, path string, resourceContent []byte) error
}
