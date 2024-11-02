package store

type Trie struct {
	root *RootNode
}

func NewTrie() *Trie {
	return &Trie{newRootNode()}
}

func (t *Trie) Insert(key, value []byte) {
	t.root.insert(key, value)
}

type Node interface {
	insert(key, value []byte)
	isLeaf() bool
}

type RootNode struct {
	Prefixes [][]byte
	Children []Node
}

func (rn *RootNode) insert(key, value []byte) {
	if len(rn.Prefixes) == 0 {
		newIntNode := newInternalNode(key)
		newIntNode.addLeafNode(key, value)
		rn.Prefixes = append(rn.Prefixes, key)
		rn.Children = append(rn.Children, newIntNode)
	}
}

func (rn *RootNode) isLeaf() bool {
	return false
}

func newRootNode() *RootNode {
	return &RootNode{}
}

type InternalNode struct {
	prefix []byte
	keys   []byte
	values []Node
	count  int
}

func (in *InternalNode) addLeafNode(key, value []byte) {
	leaf := newLeafNode(key, value)
	in.keys = append(in.keys, 0x00)
	in.values = append(in.values, leaf)
	in.count++
}

func newInternalNode(prefix []byte) *InternalNode {
	return &InternalNode{prefix: prefix}
}

func (in *InternalNode) insert(key, value []byte) {
	// First insertion into Root Node
	if in.prefix == nil && in.count == 0 {
		newIntNode := newInternalNode(key)
		newIntNode.addLeafNode(key, value)
		in.values = append(in.values, newIntNode)
		in.count++
	}
}

func (in *InternalNode) isLeaf() bool {
	return false
}

type LeafNode struct {
	key   []byte
	value []byte
}

func newLeafNode(key, value []byte) *LeafNode {
	return &LeafNode{key, value}
}

func (ln *LeafNode) Value() []byte {
	return ln.value
}

func (ln *LeafNode) insert(key, value []byte) {}

func (ln *LeafNode) isLeaf() bool {
	return true
}
