package log

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
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
	Entries   []*Entry
	NextIndex int
	Filepath  string
	mu        sync.RWMutex
}

func Init(path string) *Log {
	return &Log{Filepath: path}
}

// CommitRecords read locks l.Records, checks for new records, marshals any new records, unlocks l.Records,
// and appends the marshaled records to the json file at l.Filepath
// the log file is meant to add durability and replication such that if the program crashes, the in-memory log can be recovered upon restart
func (l *Log) CommitNewRecords() {
	l.mu.RLock()
	if l.NextIndex == len(l.Entries) {
		return
	}
	newEntries := append([]*Entry(nil), l.Entries[l.NextIndex:]...)
	marshaledEntries, err := json.Marshal(newEntries)
	if err != nil {
		fmt.Printf("Could not marshal new records to json: %s", err.Error())
		return
	}
	l.mu.RUnlock()
	logFile, err := os.OpenFile(l.Filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Error opening file %s", err.Error())
		return
	}
	defer logFile.Close()
	_, err = logFile.Write(append(marshaledEntries, '\n'))
	if err != nil {
		fmt.Printf("Error writing new records to file: %s", err.Error())
		return
	}
	l.NextIndex += len(newEntries)
}

// Entry is a single log entry for a write-ahead log that stores four items:
// Action: CREATE, UPDATE, DELETE
// Key string-typed key affected by the action
// Value can be any type. If the action is DELETE, the Value field will be nil
// Timestamp is the time the entry was created
// Term is the term of the leader node responsible for creating the entry
type Entry struct {
	Action    Action
	Key       string
	Value     any
	Timestamp time.Time
	Term      int
}

func NewRecord(action Action, key string, value any, term int) *Entry {
	return &Entry{action, key, value, time.Now(), term}
}

// Add Record adds a record to the log and timestamps it at the point the NewRecord is created
func (l *Log) AppendEntry(action Action, key string, value any) {
	l.mu.Lock()
	l.Entries = append(l.Entries, NewRecord(action, key, value, 0))
	l.mu.Unlock()
}
