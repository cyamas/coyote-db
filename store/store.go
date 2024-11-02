package store

import (
	"hash/crc32"
)

type Store struct {
	Data map[string]*Item
}

type Item struct {
	Checksum uint32
	Key      []byte
	Value    []byte
}

func NewItem(key, value []byte) *Item {
	checksum := crc32.ChecksumIEEE(value)
	return &Item{
		checksum,
		key,
		value,
	}
}
