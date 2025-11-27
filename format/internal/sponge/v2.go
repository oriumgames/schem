package sponge

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"maps"

	"github.com/oriumgames/nbt"
	"github.com/oriumgames/schem/format/internal/base"
)

// v2NBT is the NBT structure for Sponge Schematic Version 2
type v2NBT struct {
	Version         int32            `nbt:"Version"`
	DataVersion     int32            `nbt:"DataVersion"`
	Width           int16            `nbt:"Width"`
	Height          int16            `nbt:"Height"`
	Length          int16            `nbt:"Length"`
	Offset          []int32          `nbt:"Offset,array,omitempty"`
	Metadata        map[string]any   `nbt:"Metadata,omitempty"`
	PaletteMax      int32            `nbt:"PaletteMax"`
	Palette         map[string]int32 `nbt:"Palette"`
	BlockData       []byte           `nbt:"BlockData,array"`
	BlockEntities   []map[string]any `nbt:"BlockEntities,omitempty"`
	Entities        []map[string]any `nbt:"Entities,omitempty"`
	BiomePaletteMax int32            `nbt:"BiomePaletteMax,omitempty"`
	BiomePalette    map[string]int32 `nbt:"BiomePalette,omitempty"`
	BiomeData       []byte           `nbt:"BiomeData,array,omitempty"`
	Extra           map[string]any   `nbt:"*"`
}

// ReadV2 reads a Sponge Schematic v2 file.
func ReadV2(r io.Reader) (base.Schematic, error) {
	// Decompress gzip
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip decompress: %w", err)
	}
	defer gz.Close()

	var data v2NBT
	if err := nbt.NewDecoderWithEncoding(gz, nbt.BigEndian).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode nbt: %w", err)
	}

	if data.Version != 2 {
		return nil, fmt.Errorf("expected version 2, got %d", data.Version)
	}

	// Validate dimensions
	width, height, length := int(data.Width), int(data.Height), int(data.Length)
	if width <= 0 || height <= 0 || length <= 0 {
		return nil, fmt.Errorf("invalid dimensions: %dx%dx%d", width, height, length)
	}

	// Create schematic
	s := base.New(width, height, length, "sponge_v2")
	s.SetDataVersion(int(data.DataVersion))

	// Set offset
	if len(data.Offset) >= 3 {
		s.SetOffset(int(data.Offset[0]), int(data.Offset[1]), int(data.Offset[2]))
	}

	// Set metadata
	for k, v := range data.Metadata {
		s.SetMetadata(k, v)
	}

	// Build palette
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

	// Set block entities
	for _, beData := range data.BlockEntities {
		be := &base.BlockEntity{
			Data: make(map[string]any),
		}

		// Extract position
		if pos, ok := beData["Pos"].([]any); ok && len(pos) >= 3 {
			be.X = int(pos[0].(int32))
			be.Y = int(pos[1].(int32))
			be.Z = int(pos[2].(int32))
		}

		// Extract ID
		if id, ok := beData["Id"].(string); ok {
			be.ID = id
		}

		// Copy remaining data
		for k, v := range beData {
			if k != "Pos" && k != "Id" {
				be.Data[k] = v
			}
		}

		s.SetBlockEntity(be.X, be.Y, be.Z, be)
	}

	// Decode biomes (2D in v2)
	if len(data.BiomeData) > 0 {
		biomePalette := make([]string, data.BiomePaletteMax+1)
		for biomeName, idx := range data.BiomePalette {
			if int(idx) < len(biomePalette) {
				biomePalette[idx] = biomeName
			}
		}

		biomeCount := int(data.Width) * int(data.Length)
		biomeIndices, err := base.DecodeVarIntArray(data.BiomeData, biomeCount)
		if err != nil {
			return nil, fmt.Errorf("decode biome data: %w", err)
		}

		for z := 0; z < int(data.Length); z++ {
			for x := 0; x < int(data.Width); x++ {
				idx := x + z*int(data.Width)
				if idx >= len(biomeIndices) {
					continue
				}
				biomeIdx := biomeIndices[idx]
				if biomeIdx >= 0 && biomeIdx < len(biomePalette) {
					s.SetBiome(x, 0, z, biomePalette[biomeIdx])
				}
			}
		}
	}

	// Decode entities
	for _, entData := range data.Entities {
		ent := &base.Entity{
			Data: make(map[string]any),
		}

		// Extract position
		if pos, ok := entData["Pos"].([]any); ok && len(pos) >= 3 {
			ent.Pos[0] = pos[0].(float64)
			ent.Pos[1] = pos[1].(float64)
			ent.Pos[2] = pos[2].(float64)
		}

		// Extract rotation
		if rot, ok := entData["Rotation"].([]any); ok && len(rot) >= 2 {
			ent.Rotation[0] = rot[0].(float32)
			ent.Rotation[1] = rot[1].(float32)
		}

		// Extract motion
		if motion, ok := entData["Motion"].([]any); ok && len(motion) >= 3 {
			ent.Motion[0] = motion[0].(float64)
			ent.Motion[1] = motion[1].(float64)
			ent.Motion[2] = motion[2].(float64)
		}

		// Extract ID
		if id, ok := entData["Id"].(string); ok {
			ent.ID = id
		}

		// Copy remaining data
		for k, v := range entData {
			if k != "Pos" && k != "Rotation" && k != "Motion" && k != "Id" {
				ent.Data[k] = v
			}
		}

		s.AddEntity(ent)
	}

	return s, nil
}

// WriteV2 writes a schematic as Sponge Schematic v2.
func WriteV2(w io.Writer, s base.Schematic) error {
	width, height, length := s.Dimensions()
	offsetX, offsetY, offsetZ := s.Offset()

	// Build block palette
	palette := base.NewPaletteWithAir()
	blockIndices := make([]int, width*height*length)

	for y := range height {
		for z := range length {
			for x := range width {
				idx := x + z*width + y*width*length
				block := s.Block(x, y, z)
				if block == nil {
					blockIndices[idx] = 0
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
	data := v2NBT{
		Version:     2,
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

	// Encode block entities
	for y := range height {
		for z := range length {
			for x := range width {
				be := s.BlockEntity(x, y, z)
				if be == nil {
					continue
				}

				beData := make(map[string]any)
				beData["Pos"] = []int32{int32(x), int32(y), int32(z)}
				beData["Id"] = be.ID
				maps.Copy(beData, be.Data)
				data.BlockEntities = append(data.BlockEntities, beData)
			}
		}
	}

	// Encode biomes (2D)
	biomePalette := base.NewPalette()
	biomeIndices := make([]int, width*length)
	hasBiomes := false

	for z := range length {
		for x := range width {
			idx := x + z*width
			biome := s.Biome(x, 0, z)
			if biome != "" {
				hasBiomes = true
				biomeIndices[idx] = biomePalette.Add(base.BlockState{Name: biome})
			}
		}
	}

	if hasBiomes {
		biomePaletteMap := make(map[string]int32)
		for i, block := range biomePalette.Blocks() {
			biomePaletteMap[block.Name] = int32(i)
		}
		data.BiomePaletteMax = int32(biomePalette.Size() - 1)
		data.BiomePalette = biomePaletteMap
		data.BiomeData = base.EncodeVarIntArray(biomeIndices)
	}

	// Encode entities
	for _, ent := range s.Entities() {
		entData := make(map[string]any)
		entData["Pos"] = []float64{ent.Pos[0], ent.Pos[1], ent.Pos[2]}
		entData["Rotation"] = []float32{ent.Rotation[0], ent.Rotation[1]}
		entData["Motion"] = []float64{ent.Motion[0], ent.Motion[1], ent.Motion[2]}
		entData["Id"] = ent.ID
		maps.Copy(entData, ent.Data)
		data.Entities = append(data.Entities, entData)
	}

	// Compress and write
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if err := nbt.NewEncoderWithEncoding(gz, nbt.BigEndian).Encode(data); err != nil {
		return fmt.Errorf("encode nbt: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("close gzip: %w", err)
	}

	_, err := w.Write(buf.Bytes())
	return err
}
