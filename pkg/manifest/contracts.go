package manifest

// FileSource interface used by manifest package to get its nessesary information fron links
type FileSource interface {
	//ManifestFromURL Gets the manifest content from a given url
	ManifestFromURL(url string) (string, error)
	//FileTreeFromURL Get files that are present in the given url tree
	FileTreeFromURL(url string) ([]string, error)
	//BuildAbsLink Builds the abs link given where it is referenced
	BuildAbsLink(source, link string) (string, error)
}
