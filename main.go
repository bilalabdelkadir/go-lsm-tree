package main

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/bilalabdelkadir/go-lsm-tree/memtable"
	"github.com/bilalabdelkadir/go-lsm-tree/sstable"
	"github.com/bilalabdelkadir/go-lsm-tree/wal"
)

func ReplayWal(m *memtable.Memtable, w *wal.Wal) error {

	walEntries, err := w.ReadAll()
	if err != nil {
		return err
	}

	for _, walEntry := range walEntries {
		if walEntry.IsTombstone {
			m.Delete(walEntry.Key)
		} else {
			m.Put(walEntry.Key, walEntry.Value)

		}
	}

	return nil
}

func Put(m *memtable.Memtable, w *wal.Wal, key string, value string) error {

	err := w.Append(wal.WalEntry{
		Key:         key,
		Value:       value,
		IsTombstone: false,
	})

	if err != nil {
		return err
	}

	m.Put(key, value)

	return nil

}

func Delete(m *memtable.Memtable, w *wal.Wal, key string) error {
	err := w.Append(wal.WalEntry{
		Key:         key,
		IsTombstone: true,
	})

	if err != nil {
		return err
	}

	m.Delete(key)

	return nil
}

var sstables []*sstable.SsTable

func Flush(m *memtable.Memtable, w *wal.Wal) error {
	folderPath := "data/sstable"

	// get all the key from memtables
	latestData := m.GetAll()

	// check the last sstable
	sliceNumber := len(sstables)

	// create a folder if it doesn't exist so that our sstable is organized
	err := os.MkdirAll(folderPath, os.ModePerm)
	if err != nil {
		return err
	}
	// create a file inside that folder
	// File path inside the folder
	filePath := filepath.Join(folderPath, "table"+strconv.Itoa(sliceNumber)+".sst")

	var entries []sstable.SsTableEntry

	for _, data := range latestData {
		entries = append(entries, sstable.SsTableEntry{
			Key:         data.Key,
			Value:       data.Entry.Value,
			IsTombstone: data.Entry.IsTombstone,
		})
	}

	// write to the sstable
	err = sstable.WriteSSTable(filePath, entries)
	if err != nil {
		return err
	}

	// update the slice of sstables
	ssTableTopush, err := sstable.OpenSSTable(filePath)

	if err != nil {
		return err
	}

	sstables = append(sstables, ssTableTopush)

	// clear memtable and wal
	m.Clear()
	w.Clear()

	return nil

}

func Get(m *memtable.Memtable, key string) (memtable.Entry, bool) {

	if value, found := m.Get(key); found {
		if value.IsTombstone {
			return memtable.Entry{}, false
		}
		return value, true
	}

	// 2. Check SSTables (newest → oldest)
	for i := len(sstables) - 1; i >= 0; i-- {
		value, found := sstables[i].Search(key)
		if !found {
			continue
		}

		if value.IsTombstone {
			return memtable.Entry{}, false
		}

		return memtable.Entry{
			Value:       value.Value,
			IsTombstone: false,
		}, true
	}

	// 3. Not found anywhere
	return memtable.Entry{}, false
}

func LoadSsTable() {
	files, err := SortedSSTableFiles("data/sstable")
	if err != nil {
		return
	}

	for _, name := range files {
		path := "data/sstable/" + name

		sst, err := sstable.OpenSSTable(path)
		if err != nil {
			continue
		}

		sstables = append(sstables, sst)
	}
}

func SortedSSTableFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	type tableFile struct {
		id   int
		name string
	}

	var tables []tableFile

	for _, e := range entries {
		name := e.Name()

		if !strings.HasPrefix(name, "table") || !strings.HasSuffix(name, ".sst") {
			continue
		}

		numStr := strings.TrimSuffix(strings.TrimPrefix(name, "table"), ".sst")
		id, err := strconv.Atoi(numStr)
		if err != nil {
			continue
		}

		tables = append(tables, tableFile{id: id, name: name})
	}

	sort.Slice(tables, func(i, j int) bool {
		return tables[i].id < tables[j].id
	})

	result := make([]string, 0, len(tables))
	for _, t := range tables {
		result = append(result, t.name)
	}

	return result, nil
}

func Compact() {
	// find list of tables or their length
	folders, err := SortedSSTableFiles("data/sstable")
	if err != nil {
		return
	}
	//define temp var
	var temp []sstable.SsTableEntry
	// then we start a for loop
	for i := 0; i < len(folders)-1; i++ {
		// call mergeTwo in forloop

		if temp == nil {
			olderTable, _ := sstable.OpenSSTable("data/sstable/" + folders[i])
			newerTable, _ := sstable.OpenSSTable("data/sstable/" + folders[i+1])
			temp = MergeTwo(olderTable.Entries, newerTable.Entries)
		} else {
			newerTable, _ := sstable.OpenSSTable("data/sstable/" + folders[i+1])
			temp = MergeTwo(temp, newerTable.Entries)
		}
	}

	os.RemoveAll("data/sstable/")
	os.MkdirAll("data/sstable", os.ModePerm)

	// we write temp into table0.sst
	sstable.WriteSSTable("data/sstable/table0.sst", temp)

}

func MergeTwo(older []sstable.SsTableEntry, newer []sstable.SsTableEntry) []sstable.SsTableEntry {
	var result []sstable.SsTableEntry
	a, b := 0, 0

	for a < len(older) && b < len(newer) {
		if older[a].Key < newer[b].Key {
			if !older[a].IsTombstone {
				result = append(result, older[a])
			}
			a++ // always advance
		} else if older[a].Key > newer[b].Key {
			if !newer[b].IsTombstone {
				result = append(result, newer[b])
			}
			b++ // always advance
		} else { // keys are equal
			if !newer[b].IsTombstone {
				result = append(result, newer[b])
			}
			a++ // always advance both
			b++
		}
	}

	// handle leftovers
	for ; a < len(older); a++ {
		if !older[a].IsTombstone {
			result = append(result, older[a])
		}
	}

	for ; b < len(newer); b++ {
		if !newer[b].IsTombstone {
			result = append(result, newer[b])
		}
	}

	return result
}

func main() {
	// Step 1: Create test data
	newWal, _ := wal.NewWal("file.json")
	newMemtable := memtable.NewMemtable(3)

	Put(newMemtable, newWal, "a", "old_a")
	Put(newMemtable, newWal, "b", "old_b")
	Put(newMemtable, newWal, "c", "old_c")
	Flush(newMemtable, newWal) // table0: a, b, c

	Put(newMemtable, newWal, "b", "new_b") // overwrites b
	Put(newMemtable, newWal, "d", "val_d")
	Put(newMemtable, newWal, "e", "val_e")
	Flush(newMemtable, newWal) // table1: b, d, e

	log.Printf("Before compact: %d SSTables", len(sstables))

	// Step 2: Compact
	Compact()

	// Step 3: Reload and verify
	sstables = nil // clear in-memory
	LoadSsTable()
	log.Printf("After compact: %d SSTables", len(sstables))

	// Check that "b" has the new value
	if val, found := Get(newMemtable, "b"); found {
		log.Println("b =", val.Value) // should print "new_b"
	}
}
