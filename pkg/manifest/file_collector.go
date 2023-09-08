package manifest

// FileCollector collects files from manifest
type FileCollector struct {
	files []*Node
}

// Collect adds file to collection
func (fc *FileCollector) Collect(file *Node) {
	fc.files = append(fc.files, file)
}

// Extract gets files without duplication
func (fc *FileCollector) Extract() ([]*Node, error) {
	/*	exists := map[string]struct{}{}
		for _, file := range fc.files {
			if _, ok := exists[path.Join(file.Path, file.File)]; ok {
				return []*Node{}, fmt.Errorf("collision of files %s", path.Join(file.Path, file.File))
			}
			exists[path.Join(file.Path, file.File)] = struct{}{}
		}
	*/
	return fc.files, nil
}
