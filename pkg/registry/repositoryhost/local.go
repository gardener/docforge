package repositoryhost

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	ospkg "os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gardener/docforge/pkg/osfakes/httpclient"
	"github.com/gardener/docforge/pkg/osfakes/osshim"
	"github.com/gardener/docforge/pkg/osfakes/osshim/osshimfakes"
)

// Local represents a local repository defined by respurce mapping
type Local struct {
	os        osshim.Os
	urlPrefix string
	localPath string
}

// NewLocalTest creates a local repository host used for testing
func NewLocalTest(localRepo embed.FS, urlPrefix string, localPath string) Interface {
	os := &osshimfakes.FakeOs{}
	os.ReadFileCalls(localRepo.ReadFile)
	os.IsNotExistCalls(ospkg.IsNotExist)
	os.IsDirCalls(func(path string) (bool, error) {
		file, err := localRepo.Open(path)
		if err != nil {
			return false, err
		}
		stat, err := file.Stat()
		if err != nil {
			return false, err
		}
		return stat.IsDir(), nil
	})
	return &Local{os, urlPrefix, localPath}
}

// NewLocal creates a local repository host
func NewLocal(os osshim.Os, urlPrefix string, localPath string) Interface {
	return &Local{os, urlPrefix, localPath}
}

// ResourceURL returns a valid resource url object from a string url
func (l *Local) ResourceURL(resourceURL string) (*URL, error) {
	resource, err := new(resourceURL)
	if err != nil {
		return nil, err
	}
	fn := filepath.Join(l.localPath, resource.GetResourcePath())
	isDir, err := l.os.IsDir(fn)
	if err != nil {
		if l.os.IsNotExist(err) {
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
	dirPath := filepath.Join(l.localPath, resource.GetResourcePath())
	files := []string{}
	filepath.Walk(dirPath, func(path string, info fs.FileInfo, err error) error {
		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			files = append(files, strings.TrimPrefix(strings.TrimPrefix(path, dirPath), "/"))
		}
		return nil
	})
	return files, nil
}

// Accept if the link has the same url prefix as defined
func (l *Local) Accept(link string) bool {
	return strings.HasPrefix(link, l.urlPrefix)
}

// Read a resource content at uri into a byte array from file system
func (l *Local) Read(_ context.Context, resource URL) ([]byte, error) {
	fn := filepath.Join(l.localPath, resource.GetResourcePath())
	cnt, err := l.os.ReadFile(fn)
	if err != nil {
		if l.os.IsNotExist(err) {
			return nil, ErrResourceNotFound(resource.String())
		}
		if isDir, err := l.os.IsDir(fn); err == nil && isDir {
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
