package memtable

import (
	"sort"
	"sync"
)

type Entry struct {
	Value       string
	IsTombstone bool
}

type Memtable struct {
	entries     map[string]Entry
	currentSize int
	maxSize     int
	mu          sync.Mutex
}

type KeyEntry struct {
	Key   string
	Entry Entry
}

func NewMemtable(maxSize int) *Memtable {
	return &Memtable{
		entries: make(map[string]Entry),
		maxSize: maxSize,
	}
}

func (m *Memtable) Get(key string) (Entry, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.entries[key]

	if exists {
		return entry, true
	}

	return Entry{}, false

}

func (m *Memtable) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.entries[key]

	m.entries[key] = Entry{
		Value:       "",
		IsTombstone: true,
	}

	if !exists {
		m.currentSize += 1
	}
}

func (m *Memtable) GetAll() []KeyEntry {
	m.mu.Lock()
	defer m.mu.Unlock()

	keys := make([]string, 0, len(m.entries))

	for k := range m.entries {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	sortedEntry := make([]KeyEntry, 0, len(m.entries))

	for _, s := range keys {
		sortedEntry = append(sortedEntry, KeyEntry{
			Key:   s,
			Entry: m.entries[s],
		})
	}

	return sortedEntry

}

func (m *Memtable) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = make(map[string]Entry)
	m.currentSize = 0
}

func (m *Memtable) IsFull() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.currentSize >= m.maxSize
}

func (m *Memtable) Put(key string, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.entries[key]

	m.entries[key] = Entry{
		Value:       value,
		IsTombstone: false,
	}

	if !exists {
		m.currentSize += 1
	}

}
