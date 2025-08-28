package repositoryhost

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gardener/docforge/pkg/osshim/filesystem"
	"github.com/gardener/docforge/pkg/osshim/httpclient"
)

// Local represents a local repository defined by respurce mapping
type Local struct {
	filesystem filesystem.Interface
	urlPrefix  string
	localPath  string
}

// NewLocal creates a local repository host
func NewLocal(urlPrefix string, localPath string) Interface {
	return &Local{&filesystem.Local{}, urlPrefix, localPath}
}

// ResourceURL returns a valid resource url object from a string url
func (l *Local) ResourceURL(resourceURL string) (*URL, error) {
	resource, err := new(resourceURL)
	if err != nil {
		return nil, err
	}
	fn := l.filesystem.Join(l.localPath, resource.GetResourcePath())
	isDir, err := l.filesystem.IsDir(fn)
	if err != nil {
		if l.filesystem.IsNotExist(err) {
			return nil, ErrResourceNotFound(resourceURL)
		}
		return nil, err
	}
	if (isDir && resource.GetResourceType() == "blob") || (!isDir && resource.GetResourceType() == "tree") {
		return nil, ErrResourceNotFound(resourceURL)
	}
	return resource, nil
}

// ResolveRelativeLink resolves a relative link given a source resource url
func (l *Local) ResolveRelativeLink(source URL, relativeLink string) (string, error) {
	blobURL, treeURL, err := source.ResolveRelativeLink(relativeLink)
	if err != nil {
		return "", err
	}
	if _, err := l.ResourceURL(blobURL); err == nil {
		return blobURL, nil
	}
	if _, err := l.ResourceURL(treeURL); err == nil {
		return treeURL, nil
	}
	return blobURL, ErrResourceNotFound(fmt.Sprintf("%s with source %s", relativeLink, source.String()))

}

// LoadRepository does nothing
func (l *Local) LoadRepository(ctx context.Context, resourceURL string) error {
	return nil
}

// Tree returns files that are present in the given url tree
func (l *Local) Tree(resource URL) ([]string, error) {
	if resource.GetResourceType() != "tree" {
		return nil, fmt.Errorf("expected a tree url got %s", resource.String())
	}
	dirPath := l.filesystem.Join(l.localPath, resource.GetResourcePath())
	return l.filesystem.FilePathsInDir(dirPath)
}

// Accept if the link has the same url prefix as defined
func (l *Local) Accept(link string) bool {
	return strings.HasPrefix(link, strings.TrimSuffix(l.urlPrefix, "/")+"/")
}

// Read a resource content at uri into a byte array from file system
func (l *Local) Read(_ context.Context, resource URL) ([]byte, error) {
	fn := l.filesystem.Join(l.localPath, resource.GetResourcePath())
	cnt, err := l.filesystem.ReadFile(fn)
	if err != nil {
		if l.filesystem.IsNotExist(err) {
			return nil, ErrResourceNotFound(resource.String())
		}
		if isDir, err := l.filesystem.IsDir(fn); err == nil && isDir {
			return nil, fmt.Errorf("not a blob/raw url: %s", resource.String())
		}
		return nil, fmt.Errorf("reading file %s for uri %s fails: %v", fn, resource.String(), err)
	}
	return cnt, nil
}

// Name returns "local " + urlPrefix
func (l *Local) Name() string {
	return "local " + l.urlPrefix
}

// Repositories does nothing
func (l *Local) Repositories() Repositories {
	return nil
}

// GetClient does nothing
func (l *Local) GetClient() httpclient.Client {
	return nil
}

// GetRateLimit is not implemented
func (l *Local) GetRateLimit(ctx context.Context) (int, int, time.Time, error) {
	return 0, 0, time.Time{}, errors.New("not implemented")
}
