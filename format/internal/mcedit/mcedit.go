package mcedit

import (
	"compress/gzip"
	"fmt"
	"io"
	"maps"
	"strconv"
	"strings"

	"github.com/oriumgames/nbt"
	"github.com/oriumgames/pile/schem/format/internal/base"
)

type mceditNBT struct {
	Width        int16            `nbt:"Width"`
	Height       int16            `nbt:"Height"`
	Length       int16            `nbt:"Length"`
	Materials    string           `nbt:"Materials"`
	Blocks       []byte           `nbt:"Blocks,array"`
	Data         []byte           `nbt:"Data,array"`
	Entities     []map[string]any `nbt:"Entities"`
	TileEntities []map[string]any `nbt:"TileEntities"`
	TileTicks    []map[string]any `nbt:"TileTicks"`
	WEOffsetX    int32            `nbt:"WEOffsetX"`
	WEOffsetY    int32            `nbt:"WEOffsetY"`
	WEOffsetZ    int32            `nbt:"WEOffsetZ"`
	Extra        map[string]any   `nbt:"*"`
}

var reverseLegacyBlocks map[string]struct{ ID, Data byte }

func init() {
	reverseLegacyBlocks = make(map[string]struct{ ID, Data byte })
	for k, v := range legacyBlocks {
		parts := strings.Split(k, ":")
		if len(parts) != 2 {
			continue
		}
		id, err1 := strconv.Atoi(parts[0])
		data, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			continue
		}

		// Normalize the block state string for consistent key matching
		state := base.ParseBlockState(v)
		reverseLegacyBlocks[state.String()] = struct{ ID, Data byte }{byte(id), byte(data)}
	}
}

// Read reads an MCEdit/Schematica legacy schematic.
func Read(r io.Reader) (base.Schematic, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip decompress: %w", err)
	}
	defer gz.Close()

	var data mceditNBT
	if err := nbt.NewDecoderWithEncoding(gz, nbt.BigEndian).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode nbt: %w", err)
	}

	width := int(data.Width)
	height := int(data.Height)
	length := int(data.Length)

	if width <= 0 || height <= 0 || length <= 0 {
		return nil, fmt.Errorf("invalid dimensions: %dx%dx%d", width, height, length)
	}

	s := base.New(width, height, length, "mcedit")
	s.SetDataVersion(1519)
	s.SetOffset(int(data.WEOffsetX), int(data.WEOffsetY), int(data.WEOffsetZ))
	s.SetMetadata("Materials", data.Materials)

	expectedLen := width * height * length
	if len(data.Blocks) != expectedLen || len(data.Data) != expectedLen {
		return nil, fmt.Errorf("block data mismatch: expected %d bytes, got %d blocks and %d data", expectedLen, len(data.Blocks), len(data.Data))
	}

	// MCEdit format standard layout: Index = (y * Length + z) * Width + x
	for y := range height {
		for z := range length {
			for x := range width {
				idx := (y*length+z)*width + x

				if idx >= len(data.Blocks) {
					break
				}

				id := data.Blocks[idx]
				meta := data.Data[idx]

				key := fmt.Sprintf("%d:%d", id, meta)

				blockStr, ok := legacyBlocks[key]
				if !ok {
					// Fallback: try to find base block without meta
					if baseStr, ok := legacyBlocks[fmt.Sprintf("%d:0", id)]; ok {
						blockStr = baseStr
					} else {
						// Fallback to air if mapping not found
						continue
					}
				}

				state := base.ParseBlockState(blockStr)
				s.SetBlock(x, y, z, state)
			}
		}
	}

	for _, te := range data.TileEntities {
		be := &base.BlockEntity{Data: make(map[string]any)}

		if x, ok := te["x"].(int32); ok {
			be.X = int(x)
		}
		if y, ok := te["y"].(int32); ok {
			be.Y = int(y)
		}
		if z, ok := te["z"].(int32); ok {
			be.Z = int(z)
		}
		if id, ok := te["id"].(string); ok {
			be.ID = id
		}

		for k, v := range te {
			switch strings.ToLower(k) {
			case "x", "y", "z", "id":
				continue
			default:
				be.Data[k] = v
			}
		}
		s.SetBlockEntity(be.X, be.Y, be.Z, be)
	}

	for _, e := range data.Entities {
		ent := &base.Entity{Data: make(map[string]any)}

		if id, ok := e["id"].(string); ok {
			ent.ID = id
		}
		if pos, ok := e["Pos"].([]any); ok && len(pos) >= 3 {
			ent.Pos[0] = pos[0].(float64)
			ent.Pos[1] = pos[1].(float64)
			ent.Pos[2] = pos[2].(float64)
		}
		if rot, ok := e["Rotation"].([]any); ok && len(rot) >= 2 {
			ent.Rotation[0] = rot[0].(float32)
			ent.Rotation[1] = rot[1].(float32)
		}
		if mot, ok := e["Motion"].([]any); ok && len(mot) >= 3 {
			ent.Motion[0] = mot[0].(float64)
			ent.Motion[1] = mot[1].(float64)
			ent.Motion[2] = mot[2].(float64)
		}

		for k, v := range e {
			if k == "id" || k == "Pos" || k == "Rotation" || k == "Motion" {
				continue
			}
			ent.Data[k] = v
		}
		s.AddEntity(ent)
	}

	return s, nil
}

// Write writes a schematic in MCEdit legacy format.
func Write(w io.Writer, s base.Schematic) error {
	width, height, length := s.Dimensions()
	count := width * height * length
	blocks := make([]byte, count)
	data := make([]byte, count)

	for y := range height {
		for z := range length {
			for x := range width {
				idx := (y*length+z)*width + x

				state := s.Block(x, y, z)
				if state == nil {
					continue
				}

				if mapped, ok := reverseLegacyBlocks[state.String()]; ok {
					blocks[idx] = mapped.ID
					data[idx] = mapped.Data
				}
			}
		}
	}

	nbtData := mceditNBT{
		Width:     int16(width),
		Height:    int16(height),
		Length:    int16(length),
		Materials: "Alpha",
		Blocks:    blocks,
		Data:      data,
	}

	ox, oy, oz := s.Offset()
	nbtData.WEOffsetX = int32(ox)
	nbtData.WEOffsetY = int32(oy)
	nbtData.WEOffsetZ = int32(oz)

	// Block Entities
	for y := range height {
		for z := range length {
			for x := range width {
				be := s.BlockEntity(x, y, z)
				if be != nil {
					tag := make(map[string]any)
					tag["x"] = int32(x)
					tag["y"] = int32(y)
					tag["z"] = int32(z)
					tag["id"] = be.ID
					maps.Copy(tag, be.Data)
					nbtData.TileEntities = append(nbtData.TileEntities, tag)
				}
			}
		}
	}

	// Entities
	for _, ent := range s.Entities() {
		tag := make(map[string]any)
		tag["id"] = ent.ID
		tag["Pos"] = []float64{ent.Pos[0], ent.Pos[1], ent.Pos[2]}
		tag["Rotation"] = []float32{ent.Rotation[0], ent.Rotation[1]}
		tag["Motion"] = []float64{ent.Motion[0], ent.Motion[1], ent.Motion[2]}
		maps.Copy(tag, ent.Data)
		nbtData.Entities = append(nbtData.Entities, tag)
	}

	gz := gzip.NewWriter(w)
	defer gz.Close()

	if err := nbt.NewEncoderWithEncoding(gz, nbt.BigEndian).Encode(nbtData); err != nil {
		return fmt.Errorf("encode nbt: %w", err)
	}

	return nil
}
