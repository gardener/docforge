package manifest

const (
	// CachedNodeContent - key used to store Node content into properties
	CachedNodeContent = "\x00cachedNodeContent"
	// ContainerNodeSourceLocation - key used to store container Node source location into properties
	ContainerNodeSourceLocation = "\x00containerNodeSourceLocation"
	// NodeResourceSHA - key used to store Source resource SHA for later use in https://developer.github.com/v3/git/blobs/#get-a-blob
	NodeResourceSHA = "\x00nodeResourceSHA"
)
