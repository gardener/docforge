package manifest

type FileCollector struct {
	files []*Node
}

func (fc *FileCollector) Collect(file *Node) {
	fc.files = append(fc.files, file)
}

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
