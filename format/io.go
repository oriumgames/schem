package format

import (
	"bytes"
	"fmt"
	"io"
	"sort"

	"github.com/oriumgames/schem/format/internal/axiom"
	"github.com/oriumgames/schem/format/internal/litematica"
	"github.com/oriumgames/schem/format/internal/mcedit"
	"github.com/oriumgames/schem/format/internal/sponge"
)

// FormatReader is a function that reads a schematic from an io.Reader.
type FormatReader func(io.Reader) (Schematic, error)

// FormatWriter is a function that writes a schematic to an io.Writer.
type FormatWriter func(io.Writer, Schematic) error

var formatReaders = map[string]FormatReader{
	"axiom":         axiom.Read,
	"mcedit":        mcedit.Read,
	"sponge_v1":     sponge.ReadV1,
	"sponge_v2":     sponge.ReadV2,
	"sponge_v3":     sponge.ReadV3,
	"litematica_v6": litematica.ReadV6,
	"litematica_v7": litematica.ReadV7,
}

var formatWriters = map[string]FormatWriter{
	"axiom":         axiom.Write,
	"mcedit":        mcedit.Write,
	"sponge_v1":     sponge.WriteV1,
	"sponge_v2":     sponge.WriteV2,
	"sponge_v3":     sponge.WriteV3,
	"litematica_v6": litematica.WriteV6,
	"litematica_v7": litematica.WriteV7,
}

// Read reads data from r, detects the schematic format, and returns the parsed schematic.
func Read(r io.Reader) (Schematic, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read data: %w", err)
	}

	formatID, err := Detect(data)
	if err != nil {
		return nil, fmt.Errorf("detect format: %w", err)
	}

	schem, err := ReadFormat(bytes.NewReader(data), formatID)
	if err != nil {
		return nil, err
	}
	return schem, nil
}

// ReadFormat parses data from r using a specific schematic format identifier.
func ReadFormat(r io.Reader, formatID string) (Schematic, error) {
	reader, ok := formatReaders[formatID]
	if !ok {
		return nil, fmt.Errorf("unsupported format %q", formatID)
	}

	schem, err := reader(r)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", formatID, err)
	}
	return schem, nil
}

// Write writes the schematic using its native format identifier (schem.Format()).
func Write(w io.Writer, schem Schematic) error {
	formatID := schem.Format()
	if formatID == "" {
		return fmt.Errorf("schematic does not declare a format")
	}
	return WriteFormat(w, formatID, schem)
}

// WriteFormat writes the schematic using the specified format identifier.
func WriteFormat(w io.Writer, formatID string, schem Schematic) error {
	writer, ok := formatWriters[formatID]
	if !ok {
		return fmt.Errorf("unsupported format %q", formatID)
	}
	if err := writer(w, schem); err != nil {
		return fmt.Errorf("write %s: %w", formatID, err)
	}
	return nil
}

// Formats returns a sorted list of supported schematic format identifiers.
func Formats() []string {
	ids := make([]string, 0, len(formatReaders))
	for id := range formatReaders {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
