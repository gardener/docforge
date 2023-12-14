package manifest

func (n *Node) RemoveParent() {
	n.parent = nil
}
