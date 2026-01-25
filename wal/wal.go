package wal

import (
	"encoding/json"
	"io"
	"os"
)

type Wal struct {
	FilePath string
	File     *os.File
}

type WalEntry struct {
	Key         string
	Value       string
	IsTombstone bool
}

func NewWal(filePath string) (*Wal, error) {
	f, err := os.OpenFile(
		filePath,
		os.O_CREATE|os.O_APPEND|os.O_RDWR,
		0644,
	)
	if err != nil {
		return nil, err
	}
	return &Wal{
		FilePath: filePath,
		File:     f,
	}, nil

}

func (w *Wal) ReadAll() ([]WalEntry, error) {
	var allEntry []WalEntry
	w.File.Seek(0, 0)

	decoder := json.NewDecoder(w.File)

	for {
		var e WalEntry
		err := decoder.Decode(&e)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		allEntry = append(allEntry, e)

	}

	return allEntry, nil
}

func (w *Wal) Close() error {
	return w.File.Close()
}

func (w *Wal) Clear() error {
	if err := w.File.Truncate(0); err != nil {
		return err
	}

	if _, err := w.File.Seek(0, 0); err != nil {
		return err
	}

	return nil
}

func (w *Wal) Append(entry WalEntry) error {

	return json.NewEncoder(w.File).Encode(entry)

}
