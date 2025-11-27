package format

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/oriumgames/nbt"
)

const axiomMagic uint32 = 0x0AE5BB36

// Detect attempts to detect the schematic format from file data.
func Detect(data []byte) (string, error) {
	if len(data) < 4 {
		return "", fmt.Errorf("insufficient data for format detection")
	}

	// Check for Axiom Blueprint magic
	if binary.BigEndian.Uint32(data[:4]) == axiomMagic {
		return "axiom", nil
	}

	// Check for gzip magic (Sponge, Litematica, MCEdit)
	if len(data) >= 2 && data[0] == 0x1F && data[1] == 0x8B {
		return detectGzipFormat(data)
	}

	return "", fmt.Errorf("unknown format")
}

func detectGzipFormat(data []byte) (string, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("gzip decompress: %w", err)
	}
	defer gz.Close()

	nbtData, err := io.ReadAll(gz)
	if err != nil {
		return "", fmt.Errorf("read gzip data: %w", err)
	}

	// Try to decode as NBT and check root structure
	decoder := nbt.NewDecoderWithEncoding(bytes.NewReader(nbtData), nbt.BigEndian)
	var root map[string]any
	if err := decoder.Decode(&root); err != nil {
		return "", fmt.Errorf("decode nbt: %w", err)
	}

	// Check for Litematica (has "Version" and "Regions" at root)
	if version, ok := root["Version"].(int32); ok {
		if _, hasRegions := root["Regions"]; hasRegions {
			if version == 6 || version == 7 {
				return "litematica", nil
			}
			return "", fmt.Errorf("unsupported Litematica version: %d", version)
		}
	}

	// Check for Sponge Schematic (has "Version" at root)
	if version, ok := root["Version"].(int32); ok {
		switch version {
		case 1:
			return "sponge_v1", nil
		case 2:
			return "sponge_v2", nil
		case 3:
			return "sponge_v3", nil
		default:
			return "", fmt.Errorf("unknown Sponge schematic version: %d", version)
		}
	}

	// Check for MCEdit (has "Materials", "Blocks", "Data" at root)
	if _, hasMaterials := root["Materials"]; hasMaterials {
		if _, hasBlocks := root["Blocks"]; hasBlocks {
			if _, hasData := root["Data"]; hasData {
				return "mcedit", nil
			}
		}
	}

	return "", fmt.Errorf("unknown gzip NBT format")
}
