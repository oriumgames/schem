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

// v3NBT is the NBT structure for Sponge Schematic Version 3
type v3NBT struct {
	Version     int32 `nbt:"Version"`
	DataVersion int32 `nbt:"DataVersion"`

	Metadata struct {
		Name        string `nbt:"Name,omitempty"`
		Author      string `nbt:"Author,omitempty"`
		Date        int64  `nbt:"Date,omitempty"`
		Description string `nbt:"Description,omitempty"`
	} `nbt:"Metadata"`

	Width  int16 `nbt:"Width"`
	Height int16 `nbt:"Height"`
	Length int16 `nbt:"Length"`

	Offset []int32 `nbt:"Offset,omitempty"`

	Blocks struct {
		Palette       map[string]int32 `nbt:"Palette"`
		Data          []byte           `nbt:"Data"`
		BlockEntities []map[string]any `nbt:"BlockEntities,omitempty"`
	} `nbt:"Blocks"`

	Biomes struct {
		Palette []string `nbt:"Palette,omitempty"`
		Data    []byte   `nbt:"Data,omitempty"`
	} `nbt:"Biomes,omitempty"`

	Entities []map[string]any `nbt:"Entities,omitempty"`

	Extra map[string]any `nbt:"*"`
}

// ReadV3 reads a Sponge Schematic v3 file.
func ReadV3(r io.Reader) (base.Schematic, error) {
	// Decompress gzip
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip decompress: %w", err)
	}
	defer gz.Close()

	// Decode NBT
	var root struct {
		Schematic v3NBT `nbt:"Schematic"`
	}
	if err := nbt.NewDecoderWithEncoding(gz, nbt.BigEndian).Decode(&root); err != nil {
		return nil, fmt.Errorf("decode nbt: %w", err)
	}
	data := root.Schematic

	if data.Version != 3 {
		return nil, fmt.Errorf("expected version 3, got %d", data.Version)
	}

	// Validate dimensions
	width, height, length := int(data.Width), int(data.Height), int(data.Length)
	if width <= 0 || height <= 0 || length <= 0 {
		return nil, fmt.Errorf("invalid dimensions: %dx%dx%d", width, height, length)
	}

	// Create schematic
	s := base.New(width, height, length, "sponge_v3")
	s.SetDataVersion(int(data.DataVersion))

	// Set offset
	if len(data.Offset) >= 3 {
		s.SetOffset(int(data.Offset[0]), int(data.Offset[1]), int(data.Offset[2]))
	}

	// Set metadata
	s.SetMetadata("Name", data.Metadata.Name)
	s.SetMetadata("Author", data.Metadata.Author)
	s.SetMetadata("Date", data.Metadata.Date)
	s.SetMetadata("Description", data.Metadata.Description)

	// Decode blocks
	blockCount := int(data.Width) * int(data.Height) * int(data.Length)
	blockIndices, err := base.DecodeVarIntArray(data.Blocks.Data, blockCount)
	if err != nil {
		return nil, fmt.Errorf("decode block data: %w", err)
	}

	// Build palette
	palette := make([]*base.BlockState, len(data.Blocks.Palette))
	for stateStr, id := range data.Blocks.Palette {
		if int(id) >= len(palette) {
			newPalette := make([]*base.BlockState, int(id)+1)
			copy(newPalette, palette)
			palette = newPalette
		}
		palette[id] = base.ParseBlockState(stateStr)
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
	for _, beData := range data.Blocks.BlockEntities {
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

	// Decode biomes (3D)
	if len(data.Biomes.Data) > 0 && len(data.Biomes.Palette) > 0 {
		biomeCount := int(data.Width) * int(data.Height) * int(data.Length)
		biomeIndices, err := base.DecodeVarIntArray(data.Biomes.Data, biomeCount)
		if err != nil {
			return nil, fmt.Errorf("decode biome data: %w", err)
		}

		for y := 0; y < int(data.Height); y++ {
			for z := 0; z < int(data.Length); z++ {
				for x := 0; x < int(data.Width); x++ {
					idx := x + z*int(data.Width) + y*int(data.Width)*int(data.Length)
					if idx >= len(biomeIndices) {
						continue
					}
					biomeIdx := biomeIndices[idx]
					if biomeIdx >= 0 && biomeIdx < len(data.Biomes.Palette) {
						s.SetBiome(x, y, z, data.Biomes.Palette[biomeIdx])
					}
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

// WriteV3 writes a schematic as Sponge Schematic v3.
func WriteV3(w io.Writer, s base.Schematic) error {
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
					blockIndices[idx] = 0
				} else {
					blockIndices[idx] = palette.Add(*block)
				}
			}
		}
	}

	// Build NBT structure
	data := v3NBT{
		Version:     3,
		DataVersion: int32(s.DataVersion()),
		Width:       int16(width),
		Height:      int16(height),
		Length:      int16(length),
		Offset:      []int32{int32(offsetX), int32(offsetY), int32(offsetZ)},
	}

	// Metadata
	meta := s.Metadata()
	if name, ok := meta["Name"].(string); ok {
		data.Metadata.Name = name
	}
	if author, ok := meta["Author"].(string); ok {
		data.Metadata.Author = author
	}
	if date, ok := meta["Date"].(int64); ok {
		data.Metadata.Date = date
	}
	if desc, ok := meta["Description"].(string); ok {
		data.Metadata.Description = desc
	}

	// Encode palette
	data.Blocks.Palette = make(map[string]int32, palette.Size())
	for i, block := range palette.Blocks() {
		data.Blocks.Palette[block.String()] = int32(i)
	}

	// Encode block data
	data.Blocks.Data = base.EncodeVarIntArray(blockIndices)

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
				data.Blocks.BlockEntities = append(data.Blocks.BlockEntities, beData)
			}
		}
	}

	// Encode biomes (3D)
	biomePalette := base.NewPalette()
	biomeIndices := make([]int, width*height*length)
	hasBiomes := false

	for y := range height {
		for z := range length {
			for x := range width {
				idx := x + z*width + y*width*length
				biome := s.Biome(x, y, z)
				if biome != "" {
					hasBiomes = true
					biomeIndices[idx] = biomePalette.Add(base.BlockState{Name: biome})
				}
			}
		}
	}

	if hasBiomes {
		data.Biomes.Palette = make([]string, biomePalette.Size())
		for i, block := range biomePalette.Blocks() {
			data.Biomes.Palette[i] = block.Name
		}
		data.Biomes.Data = base.EncodeVarIntArray(biomeIndices)
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

	// Wrap in root tag
	root := struct {
		Schematic v3NBT `nbt:"Schematic"`
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
