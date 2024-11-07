package log

import (
	"sync"

	coyote_db "github.com/cyamas/coyote-db/proto"
)

type Action int

const (
	CREATE = iota
	UPDATE
	DELETE
)

// Log is an in-memory append-only history of changes to the key-value store
// It serves as a write-ahead-log to enable new instances of the key-value store to get up to speed
// New records will periodically be written to a json file for durability
type Log struct {
	Entries   []*coyote_db.Entry
	NextIndex int
	Filepath  string
	mu        sync.RWMutex
}

func Init(path string) *Log {
	return &Log{Filepath: path}
}

func (l *Log) Lock() {
	l.mu.Lock()
}

func (l *Log) Unlock() {
	l.mu.Lock()
}

func (l *Log) RLock() {
	l.mu.RLock()
}

func (l *Log) RUnlock() {
	l.mu.RUnlock()
}
