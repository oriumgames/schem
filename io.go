package schem

import (
	"io"
	"os"

	"github.com/oriumgames/pile/schem/format"
)

// Read reads a schematic file with auto-format detection.
func Read(r io.Reader) (*Structure, error) {
	s, err := format.Read(r)
	if err != nil {
		return nil, err
	}
	return NewStructure(s), nil
}

// ReadFile reads a schematic from a file path.
func ReadFile(path string) (*Structure, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Read(f)
}

// ReadFormat reads a schematic with a specific format.
func ReadFormat(r io.Reader, formatID string) (*Structure, error) {
	s, err := format.ReadFormat(r, formatID)
	if err != nil {
		return nil, err
	}
	return NewStructure(s), nil
}

// Write writes the structure in its native format.
func Write(w io.Writer, s *Structure) error {
	return format.Write(w, s.schematic)
}

// WriteFile writes the structure to a file.
func WriteFile(path string, s *Structure) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return Write(f, s)
}

// WriteFormat writes the structure in the specified format.
func WriteFormat(w io.Writer, formatID string, s *Structure) error {
	return format.WriteFormat(w, formatID, s.schematic)
}

// Formats returns a list of supported format identifiers.
func Formats() []string {
	return format.Formats()
}
