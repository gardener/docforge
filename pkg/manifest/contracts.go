package manifest

type FileSource interface {
	// gets the manifest content from a given url
	//ManifestFromUrl(url string) (string, error)
	//TODO use this instead of ManifestFromUrl ?
	ManifestFromUrl(url string) (string, error)

	FileTreeFromUrl(url string) ([]string, error)

	BuildAbsLink(source, link string) (string, error)
}
