package sstable

import (
	"encoding/json"
	"io"
	"os"
	"sort"
)

type SsTable struct {
	FilePath string
	Entries  []SsTableEntry
}

type SsTableEntry struct {
	Key         string
	Value       string
	IsTombstone bool
}

func WriteSSTable(filePath string, entries []SsTableEntry) error {
	f, err := os.OpenFile(
		filePath,
		os.O_CREATE|os.O_RDWR,
		0644,
	)
	if err != nil {
		return err
	}

	defer f.Close()

	encoder := json.NewEncoder(f)
	for _, entry := range entries {
		err = encoder.Encode(entry)
		if err != nil {
			return err
		}
	}

	return nil
}

func OpenSSTable(filePath string) (*SsTable, error) {
	f, err := os.OpenFile(
		filePath,
		os.O_RDONLY,
		0644,
	)
	if err != nil {
		return nil, err
	}

	defer f.Close()
	var allEntry []SsTableEntry

	decoder := json.NewDecoder(f)

	for {
		var e SsTableEntry
		err := decoder.Decode(&e)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		allEntry = append(allEntry, e)

	}

	return &SsTable{
		Entries:  allEntry,
		FilePath: filePath,
	}, nil

}

func (s *SsTable) Search(target string) (SsTableEntry, bool) {
	index := sort.Search(len(s.Entries), func(i int) bool {
		return s.Entries[i].Key >= target
	})

	if index < len(s.Entries) && s.Entries[index].Key == target {
		return s.Entries[index], true
	}

	return SsTableEntry{}, false
}
