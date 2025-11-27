package sponge

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"maps"

	"github.com/oriumgames/nbt"
	"github.com/oriumgames/pile/schem/format/internal/base"
)

// v1NBT is the NBT structure for Sponge Schematic Version 1
type v1NBT struct {
	Version      int32            `nbt:"Version"`
	DataVersion  int32            `nbt:"DataVersion"`
	Width        int16            `nbt:"Width"`
	Height       int16            `nbt:"Height"`
	Length       int16            `nbt:"Length"`
	Offset       []int32          `nbt:"Offset,omitempty"`
	Metadata     map[string]any   `nbt:"Metadata,omitempty"`
	PaletteMax   int32            `nbt:"PaletteMax"`
	Palette      map[string]int32 `nbt:"Palette"`
	BlockData    []byte           `nbt:"BlockData"`
	TileEntities []map[string]any `nbt:"TileEntities,omitempty"`
	Extra        map[string]any   `nbt:"*"`
}

// ReadV1 reads a Sponge Schematic v1 file.
func ReadV1(r io.Reader) (base.Schematic, error) {
	// Decompress gzip
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip decompress: %w", err)
	}
	defer gz.Close()

	// Decode NBT
	var data v1NBT
	if err := nbt.NewDecoderWithEncoding(gz, nbt.BigEndian).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode nbt: %w", err)
	}

	if data.Version != 1 {
		return nil, fmt.Errorf("expected version 1, got %d", data.Version)
	}

	// Validate dimensions
	width, height, length := int(data.Width), int(data.Height), int(data.Length)
	if width <= 0 || height <= 0 || length <= 0 {
		return nil, fmt.Errorf("invalid dimensions: %dx%dx%d", width, height, length)
	}

	// Create schematic
	s := base.New(width, height, length, "sponge_v1")
	s.SetDataVersion(int(data.DataVersion))

	// Set offset
	if len(data.Offset) >= 3 {
		s.SetOffset(int(data.Offset[0]), int(data.Offset[1]), int(data.Offset[2]))
	}

	// Set metadata
	for k, v := range data.Metadata {
		s.SetMetadata(k, v)
	}

	// Build palette (inverted: string -> index)
	palette := make([]*base.BlockState, data.PaletteMax+1)
	for blockName, idx := range data.Palette {
		if int(idx) < len(palette) {
			palette[idx] = base.ParseBlockState(blockName)
		}
	}

	// Decode blocks
	blockCount := int(data.Width) * int(data.Height) * int(data.Length)
	blockIndices, err := base.DecodeVarIntArray(data.BlockData, blockCount)
	if err != nil {
		return nil, fmt.Errorf("decode block data: %w", err)
	}

	// Set blocks
	for y := 0; y < int(data.Height); y++ {
		for z := 0; z < int(data.Length); z++ {
			for x := 0; x < int(data.Width); x++ {
				idx := x + z*int(data.Width) + y*int(data.Width)*int(data.Length)
				if idx >= len(blockIndices) {
					continue
				}
				paletteIdx := blockIndices[idx]
				if paletteIdx >= 0 && paletteIdx < len(palette) && palette[paletteIdx] != nil {
					s.SetBlock(x, y, z, palette[paletteIdx].Clone())
				}
			}
		}
	}

	// Set tile entities (v1 uses "TileEntities" not "BlockEntities")
	for _, teData := range data.TileEntities {
		be := &base.BlockEntity{
			Data: make(map[string]any),
		}

		// Extract position
		if pos, ok := teData["Pos"].([]any); ok && len(pos) >= 3 {
			be.X = int(pos[0].(int32))
			be.Y = int(pos[1].(int32))
			be.Z = int(pos[2].(int32))
		}

		// Extract ID
		if id, ok := teData["Id"].(string); ok {
			be.ID = id
		}

		// Copy remaining data
		for k, v := range teData {
			if k != "Pos" && k != "Id" {
				be.Data[k] = v
			}
		}

		s.SetBlockEntity(be.X, be.Y, be.Z, be)
	}

	return s, nil
}

// WriteV1 writes a schematic as Sponge Schematic v1.
func WriteV1(w io.Writer, s base.Schematic) error {
	width, height, length := s.Dimensions()
	offsetX, offsetY, offsetZ := s.Offset()

	// Build palette
	palette := base.NewPaletteWithAir()
	blockIndices := make([]int, width*height*length)

	for y := range height {
		for z := range length {
			for x := range width {
				idx := x + z*width + y*width*length
				block := s.Block(x, y, z)
				if block == nil {
					blockIndices[idx] = 0 // Air
				} else {
					blockIndices[idx] = palette.Add(*block)
				}
			}
		}
	}

	// Build palette map
	paletteMap := make(map[string]int32)
	for i, block := range palette.Blocks() {
		paletteMap[block.String()] = int32(i)
	}

	// Build NBT structure
	data := v1NBT{
		Version:     1,
		DataVersion: int32(s.DataVersion()),
		Width:       int16(width),
		Height:      int16(height),
		Length:      int16(length),
		Offset:      []int32{int32(offsetX), int32(offsetY), int32(offsetZ)},
		PaletteMax:  int32(palette.Size() - 1),
		Palette:     paletteMap,
		BlockData:   base.EncodeVarIntArray(blockIndices),
		Metadata:    s.Metadata(),
	}

	// Encode tile entities
	for y := range height {
		for z := range length {
			for x := range width {
				be := s.BlockEntity(x, y, z)
				if be == nil {
					continue
				}

				teData := make(map[string]any)
				teData["Pos"] = []int32{int32(x), int32(y), int32(z)}
				teData["Id"] = be.ID
				maps.Copy(teData, be.Data)
				data.TileEntities = append(data.TileEntities, teData)
			}
		}
	}

	// Wrap in root tag
	root := struct {
		Schematic v1NBT `nbt:"Schematic"`
	}{Schematic: data}

	// Compress and write
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if err := nbt.NewEncoderWithEncoding(gz, nbt.BigEndian).Encode(root); err != nil {
		return fmt.Errorf("encode nbt: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("close gzip: %w", err)
	}

	_, err := w.Write(buf.Bytes())
	return err
}
