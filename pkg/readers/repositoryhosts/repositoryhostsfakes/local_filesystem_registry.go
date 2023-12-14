package repositoryhostsfakes

import "embed"

func FilesystemRegistry(dir embed.FS) *FakeRegistry {
	localHost := FakeRepositoryHost{}
	localHost.ManifestFromURLCalls(func(url string) (string, error) {
		content, err := dir.ReadFile(url)
		return string(content), err
	})
	localHost.ToAbsLinkCalls(func(url, link string) (string, error) {
		return link, nil
	})
	registry := &FakeRegistry{}
	registry.GetReturns(&localHost, nil)
	return registry
}
