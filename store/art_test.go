package store

import (
	"testing"
)

func TestInsertInitialKVPair(t *testing.T) {
	trie := NewTrie()
	trie.Insert([]byte("apple"), []byte("fruit"))
	if trie.root == nil {
		t.Fatalf("root should not be nil")
	}
	if trie.root.isLeaf() {
		t.Fatalf("root should not be leaf node")
	}
	if len(trie.root.Prefixes) != 1 || len(trie.root.Children) != 1 {
		t.Fatalf("root Prefix and root Children should have length 1")
	}
	if string(trie.root.Prefixes[0]) != "apple" {
		t.Fatalf("trie.root.Prefixes[0] should be []byte('apple')")
	}
	if trie.root.Children[0].isLeaf() {
	}
	intNode, ok := trie.root.Children[0].(*InternalNode)
	if !ok {
		t.Fatalf("root's first child should be an internal node")
	}
	if string(intNode.prefix) != "apple" {
		t.Fatalf("intNode.prefix should be 'apple'")
	}
	if intNode.count != 1 {
		t.Fatalf("intNode.count should be 1")
	}
	if intNode.keys[0] != 0x00 {
		t.Fatalf("intNode.keys[0] should be 0x00")
	}
	leaf, ok := intNode.values[0].(*LeafNode)
	if !ok {
		t.Fatalf("intNode's first value should be a leaf node")
	}
	if string(leaf.key) != "apple" {
		t.Fatalf("leaf key should be apple")
	}
	if string(leaf.value) != "fruit" {
		t.Fatalf("leaf value should be fruit")
	}
}
