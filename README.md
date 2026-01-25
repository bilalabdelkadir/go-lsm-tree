# LSM-Tree Go

A Log-Structured Merge Tree (LSM-Tree) implementation in Go. LSM-trees are write-optimized data structures commonly used in modern databases like LevelDB, RocksDB, and Cassandra.

## What is an LSM-Tree?

An LSM-tree optimizes write performance by buffering writes in memory (memtable) and periodically flushing them to immutable sorted files on disk (SSTables). Reads check the memtable first, then search through SSTables from newest to oldest. Compaction periodically merges SSTables to reclaim space and improve read performance.

## Features

- **Write-Ahead Log (WAL)** - Durability through crash recovery by logging operations before applying them
- **In-memory Memtable** - Fast writes with thread-safe operations using mutex locks
- **Persistent SSTables** - Sorted string tables stored on disk with binary search for efficient lookups
- **Tombstone-based Deletion** - Logical deletes that get cleaned up during compaction
- **Compaction** - Merges multiple SSTables into one, removing obsolete entries and tombstones

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Write Path                           │
│  Client ──► WAL (append) ──► Memtable ──► SSTable (flush)  │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                        Read Path                            │
│  Client ──► Memtable ──► SSTable[n] ──► ... ──► SSTable[0] │
│              (newest)     (newest)              (oldest)    │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                       Compaction                            │
│  SSTable[0] + SSTable[1] + ... ──► Merged SSTable          │
│  (removes tombstones and duplicate keys)                    │
└─────────────────────────────────────────────────────────────┘
```

### Write Path

1. Entry is appended to WAL for durability
2. Entry is written to the in-memory memtable
3. When memtable is full, it's flushed to a new SSTable on disk
4. WAL and memtable are cleared after successful flush

### Read Path

1. Check memtable first (most recent data)
2. Search SSTables from newest to oldest
3. Return first match found (or not found if key doesn't exist)
4. Tombstones indicate deleted keys

## Installation

```bash
go get github.com/bilalabdelkadir/lsm-tree-go
```

Or clone the repository:

```bash
git clone https://github.com/bilalabdelkadir/lsm-tree-go.git
cd lsm-tree-go
go mod tidy
```

## Usage

### Basic Example

```go
package main

import (
    "log"
    "lsm-tree-go/memtable"
    "lsm-tree-go/wal"
)

func main() {
    // Initialize WAL and Memtable
    w, _ := wal.NewWal("data/wal.json")
    m := memtable.NewMemtable(1000) // max 1000 entries before flush

    // Write data
    Put(m, w, "user:1", "alice")
    Put(m, w, "user:2", "bob")
    Put(m, w, "user:3", "charlie")

    // Read data
    if entry, found := Get(m, "user:1"); found {
        log.Println("Found:", entry.Value) // Output: Found: alice
    }

    // Delete data
    Delete(m, w, "user:2")

    // Flush memtable to SSTable
    Flush(m, w)

    // Compact SSTables
    Compact()
}
```

### Recovery After Crash

```go
// On startup, replay WAL to restore memtable state
w, _ := wal.NewWal("data/wal.json")
m := memtable.NewMemtable(1000)

// Replay any uncommitted entries from WAL
ReplayWal(m, w)

// Load existing SSTables
LoadSsTable()
```

## API Reference

### WAL Package

| Function                                | Description                    |
| --------------------------------------- | ------------------------------ |
| `NewWal(filePath string) (*Wal, error)` | Creates or opens a WAL file    |
| `(*Wal) Append(entry WalEntry) error`   | Appends an entry to the WAL    |
| `(*Wal) ReadAll() ([]WalEntry, error)`  | Reads all entries from the WAL |
| `(*Wal) Clear() error`                  | Truncates the WAL file         |
| `(*Wal) Close() error`                  | Closes the WAL file            |

**WalEntry struct:**

```go
type WalEntry struct {
    Key         string
    Value       string
    IsTombstone bool
}
```

### Memtable Package

| Function                                    | Description                                    |
| ------------------------------------------- | ---------------------------------------------- |
| `NewMemtable(maxSize int) *Memtable`        | Creates a new memtable with specified capacity |
| `(*Memtable) Put(key, value string)`        | Inserts or updates a key-value pair            |
| `(*Memtable) Get(key string) (Entry, bool)` | Retrieves an entry by key                      |
| `(*Memtable) Delete(key string)`            | Marks a key as deleted (tombstone)             |
| `(*Memtable) GetAll() []KeyEntry`           | Returns all entries sorted by key              |
| `(*Memtable) Clear()`                       | Removes all entries                            |
| `(*Memtable) IsFull() bool`                 | Returns true if memtable has reached max size  |

**Entry struct:**

```go
type Entry struct {
    Value       string
    IsTombstone bool
}
```

### SSTable Package

| Function                                                      | Description                          |
| ------------------------------------------------------------- | ------------------------------------ |
| `WriteSSTable(filePath string, entries []SsTableEntry) error` | Writes entries to an SSTable file    |
| `OpenSSTable(filePath string) (*SsTable, error)`              | Opens and loads an SSTable from disk |
| `(*SsTable) Search(target string) (SsTableEntry, bool)`       | Binary search for a key              |

**SsTableEntry struct:**

```go
type SsTableEntry struct {
    Key         string
    Value       string
    IsTombstone bool
}
```

### Main Package (Orchestration)

| Function                                            | Description                        |
| --------------------------------------------------- | ---------------------------------- |
| `Put(m *Memtable, w *Wal, key, value string) error` | Writes to WAL then memtable        |
| `Get(m *Memtable, key string) (Entry, bool)`        | Reads from memtable, then SSTables |
| `Delete(m *Memtable, w *Wal, key string) error`     | Marks key as deleted               |
| `Flush(m *Memtable, w *Wal) error`                  | Flushes memtable to SSTable        |
| `Compact()`                                         | Merges all SSTables into one       |
| `ReplayWal(m *Memtable, w *Wal) error`              | Rebuilds memtable from WAL         |
| `LoadSsTable()`                                     | Loads all SSTables from disk       |

## How It Works

### Write Operations

```
Put("name", "alice")
    │
    ▼
┌─────────┐
│   WAL   │  ← Append {"Key":"name","Value":"alice","IsTombstone":false}
└─────────┘
    │
    ▼
┌──────────┐
│ Memtable │  ← Store in hash map: entries["name"] = {Value:"alice"}
└──────────┘
```

### Read Operations

```
Get("name")
    │
    ▼
┌──────────┐
│ Memtable │  ← Check hash map first
└──────────┘
    │ (not found)
    ▼
┌───────────┐
│ SSTable N │  ← Binary search (newest)
└───────────┘
    │ (not found)
    ▼
┌───────────┐
│ SSTable 0 │  ← Binary search (oldest)
└───────────┘
```

### Compaction Process

```
Before:
  table0.sst: [a:1, b:2, c:3]
  table1.sst: [b:5, d:4]        ← b was updated
  table2.sst: [c:TOMBSTONE]     ← c was deleted

After Compact():
  table0.sst: [a:1, b:5, d:4]   ← merged, c removed, b updated
```

Compaction:

1. Reads all SSTables from oldest to newest
2. Merges entries, keeping only the newest value for each key
3. Removes tombstones (deleted entries)
4. Writes a single merged SSTable
5. Deletes old SSTable files

## File Structure

```
lsm-tree-go/
├── main.go           # Orchestration and entry point
├── memtable/
│   └── memtable.go   # In-memory sorted map
├── sstable/
│   └── sstable.go    # Persistent sorted files
├── wal/
│   └── wal.go        # Write-ahead log
└── data/
    ├── wal.json      # WAL file
    └── sstable/
        ├── table0.sst
        ├── table1.sst
        └── ...
```

## Limitations

- **No bloom filters** - Every SSTable may be searched during reads
- **No block indexing** - Entire SSTable is loaded into memory
- **Single-threaded compaction** - Compaction blocks other operations
- **No compression** - Data stored as plain JSON
- **No leveled compaction** - All SSTables merged into one
- **String keys/values only** - No support for binary data
- **No automatic flush** - Must manually call `Flush()` when memtable is full
