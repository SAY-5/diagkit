// Package history is a lightweight store for past incidents: a directory of
// archived bundle files plus an index.json describing them. The Go side
// appends to it with `diagkit archive`; the Python analyzer reads it to list
// past incidents and find signatures that recur across them.
package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/SAY-5/diagkit/internal/bundle"
)

// IndexVersion identifies the index layout. Both languages check it.
const IndexVersion = "1"

// IndexFile is the name of the index inside a history directory.
const IndexFile = "index.json"

// DefaultDir is where incidents land when no directory is given.
const DefaultDir = "diagkit-history"

// Entry summarizes one archived incident.
type Entry struct {
	ID         string        `json:"id"`
	File       string        `json:"file"`
	Scenario   string        `json:"scenario"`
	Seed       int64         `json:"seed"`
	Window     bundle.Window `json:"window"`
	Logs       int           `json:"logs"`
	Traces     int           `json:"traces"`
	Signatures int           `json:"signatures"`
}

// Index is the full history catalog.
type Index struct {
	Version   string  `json:"version"`
	Incidents []Entry `json:"incidents"`
}

// Load reads the index from dir. A missing directory or index yields an empty
// index, so the first archive call bootstraps the store.
func Load(dir string) (*Index, error) {
	data, err := os.ReadFile(filepath.Join(dir, IndexFile))
	if errors.Is(err, fs.ErrNotExist) {
		return &Index{Version: IndexVersion}, nil
	}
	if err != nil {
		return nil, err
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse %s: %w", IndexFile, err)
	}
	if idx.Version != IndexVersion {
		return nil, fmt.Errorf("unsupported history index version %q, expected %q", idx.Version, IndexVersion)
	}
	return &idx, nil
}

// Archive copies the bundle into dir under the next sequential id and appends
// it to the index. The same incident archived twice becomes two entries; the
// recurrence report on the Python side is built on exactly that.
func Archive(dir string, b *bundle.Bundle) (Entry, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Entry{}, err
	}
	idx, err := Load(dir)
	if err != nil {
		return Entry{}, err
	}

	id := fmt.Sprintf("inc-%04d", len(idx.Incidents)+1)
	e := Entry{
		ID:         id,
		File:       id + ".json",
		Scenario:   b.Scenario,
		Seed:       b.Seed,
		Window:     b.Window,
		Logs:       len(b.Logs),
		Traces:     len(b.Traces),
		Signatures: len(b.Signatures),
	}

	f, err := os.Create(filepath.Join(dir, e.File))
	if err != nil {
		return Entry{}, err
	}
	if err := b.Write(f); err != nil {
		f.Close()
		return Entry{}, err
	}
	if err := f.Close(); err != nil {
		return Entry{}, err
	}

	idx.Incidents = append(idx.Incidents, e)
	return e, writeIndex(dir, idx)
}

func writeIndex(dir string, idx *Index) error {
	f, err := os.Create(filepath.Join(dir, IndexFile))
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(idx)
}
