package manifest

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/gardener/docforge/pkg/util/urls"
	"gopkg.in/yaml.v2"
)

type NodeTransformation func(node *Node, parent *Node, manifest *Node, fs FileSource, fc *FileCollector) error

func processManifest(f NodeTransformation, node *Node, parent *Node, manifest *Node, fs FileSource, fc *FileCollector) error {
	if err := f(node, parent, manifest, fs, fc); err != nil {
		return err
	}
	manifestNode := manifest
	if node.Manifest != "" {
		manifestNode = node
	}
	for _, child := range node.Structure {
		if err := processManifest(f, child, node, manifestNode, fs, fc); err != nil {
			if node.Manifest != "" {
				return fmt.Errorf("manifest %s -> %w", node.Manifest, err)
			}
			return err
		}
	}
	return nil
}

func loadManifestStructure(node *Node, _ *Node, manifest *Node, fs FileSource, _ *FileCollector) error {
	var (
		err         error
		content     string
		newManifest string
	)
	if node.Manifest == "" {
		return nil
	}
	if newManifest, err = fs.BuildAbsLink(manifest.Manifest, node.Manifest); err != nil {
		return fmt.Errorf("can't build manifest node %s absolute URL : %w ", node.Manifest, err)
	}
	node.Manifest = newManifest
	if content, err = fs.ManifestFromUrl(node.Manifest); err != nil {
		return fmt.Errorf("can't get manifest file content : %w", err)
	}
	if err = yaml.Unmarshal([]byte(content), node); err != nil {
		return fmt.Errorf("can't parse manifest %s yaml content : %w", node.Manifest, err)
	}
	return nil
}

func decideNodeType(node *Node, _ *Node, _ *Node, _ FileSource, _ *FileCollector) error {
	node.Type = ""
	candidateType := []string{}
	if node.Manifest != "" {
		candidateType = append(candidateType, "manifest")
	}
	if node.File != "" {
		candidateType = append(candidateType, "file")
	}
	if node.Dir != "" {
		candidateType = append(candidateType, "dir")
	}
	if node.Files != "" {
		candidateType = append(candidateType, "fileTree")
	}
	if len(candidateType) != 1 {
		return fmt.Errorf("there is a node with directiry path \"%s\" with multiple types [%s]. If [] then that node doesn't have a name or doesn't have any of the properties manifest, file, files, dir", node.Path, strings.Join(candidateType, ", "))
	}
	node.Type = candidateType[0]
	return nil
}

func calculatePath(node *Node, parent *Node, _ *Node, _ FileSource, _ *FileCollector) error {
	if parent == nil {
		return nil
	}
	if parent.Path == "" {
		node.Path = "."
		return nil
	}
	switch parent.Type {
	case "dir":
		node.Path = path.Join(parent.Path, parent.Dir)
	case "manifest":
		node.Path = parent.Path
	default:
		return fmt.Errorf("parent node %s is not a dir or manifest", node.Path)
	}
	return nil
}

func resolveFileRelativeLinks(node *Node, _ *Node, manifest *Node, fs FileSource, _ *FileCollector) error {
	var (
		err     error
		newLink string
	)
	switch node.Type {
	case "file":
		// Don't calculate source for empty _index.md file
		if node.File == "_index.md" && node.Source == "" {
			return nil
		}
		if strings.Contains(node.File, "/") {
			node.Source = node.File
			node.File = path.Base(node.File)
		}
		if newLink, err = fs.BuildAbsLink(manifest.Manifest, node.Source); err != nil {
			return fmt.Errorf("cant build node's absolute link %s", node.Source)
		}
		node.Source = newLink
	case "fileTree":
		if newLink, err = fs.BuildAbsLink(manifest.Manifest, node.Files); err != nil {
			return fmt.Errorf("cant build node's absolute link %s", node.Files)
		}
		node.Files = newLink
	}
	return nil
}

func extractFilesFromNode(node *Node, _ *Node, manifest *Node, fs FileSource, fc *FileCollector) error {
	switch node.Type {
	case "file":
		if !strings.HasSuffix(node.File, ".md") {
			node.File += ".md"
		}
		fc.Collect(node)
	case "fileTree":
		files, _ := fs.FileTreeFromUrl(node.Files)
		for _, file := range files {
			extension := urls.Ext(file)
			if extension == "md" || extension == "" {
				source, err := url.JoinPath(strings.Replace(node.Files, "/tree/", "/blob/", 1), file)
				if err != nil {
					return err
				}
				fileName := path.Base(file)
				if !strings.HasSuffix(fileName, ".md") {
					fileName = fileName + ".md"
				}
				fc.Collect(&Node{
					// TODO:
					FileType: FileType{
						File:   fileName,
						Source: source,
					},
					Type: "file",
					Path: path.Join(node.Path, path.Dir(file)),
				})
			}
		}
	case "dir":
		if node.Properties != nil {
			fc.Collect(node)
		}
	}
	return nil
}

func ResolveManifest(url string, fs FileSource, fc *FileCollector) error {
	manifest := Node{
		ManifestType: ManifestType{
			Manifest: url,
		},
	}
	if err := processManifest(loadManifestStructure, &manifest, nil, &manifest, fs, fc); err != nil {
		return err
	}
	if err := processManifest(decideNodeType, &manifest, nil, &manifest, fs, fc); err != nil {
		return err
	}
	if err := processManifest(calculatePath, &manifest, nil, &manifest, fs, fc); err != nil {
		return err
	}
	if err := processManifest(resolveFileRelativeLinks, &manifest, nil, &manifest, fs, fc); err != nil {
		return err
	}
	if err := processManifest(extractFilesFromNode, &manifest, nil, &manifest, fs, fc); err != nil {
		return err
	}
	return nil
}
