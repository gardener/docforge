package manifest

type Manifest struct {
	Node

	URL string
}

type FileType struct {
	File string `yaml:"file,omitempty"`

	Source string `yaml:"source,omitempty"`

	MultiSource []string `yaml:"multiSource,omitempty"`
}

type DirType struct {
	Dir string `yaml:"dir,omitempty"`

	Structure []*Node `yaml:"structure,omitempty"`
}

type FilesType struct {
	Files string `yaml:"fileTree,omitempty"`

	ExcludeFrontMatter string `yaml:"excludeFrontMatter,omitempty"`
}

type ManifestType struct {
	Manifest string `yaml:"manifest,omitempty"`

	manifest *Manifest
}

type Node struct {
	ManifestType `yaml:",inline"`

	FileType `yaml:",inline"`

	DirType `yaml:",inline"`

	FilesType `yaml:",inline"`

	Properties map[string]interface{} `yaml:"properties,omitempty"`

	Type string `yaml:"type,omitempty"`

	Path string `yaml:"path,omitempty"`
}
