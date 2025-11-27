package litematica

import (
	"compress/gzip"
	"fmt"
	"io"
	"maps"
	"math"
	"math/bits"

	"github.com/oriumgames/nbt"
	"github.com/oriumgames/pile/schem/format/internal/base"
)

// litematicaNBT represents the main NBT structure
type litematicaNBT struct {
	Version              int32 `nbt:"Version"`
	SubVersion           int32 `nbt:"SubVersion,omitempty"`
	MinecraftDataVersion int32 `nbt:"MinecraftDataVersion"`

	Metadata struct {
		Name          string `nbt:"Name"`
		Author        string `nbt:"Author"`
		Description   string `nbt:"Description"`
		TimeCreated   int64  `nbt:"TimeCreated"`
		TimeModified  int64  `nbt:"TimeModified"`
		RegionCount   int32  `nbt:"RegionCount"`
		TotalBlocks   int32  `nbt:"TotalBlocks"`
		TotalVolume   int32  `nbt:"TotalVolume"`
		EnclosingSize struct {
			X int32 `nbt:"x"`
			Y int32 `nbt:"y"`
			Z int32 `nbt:"z"`
		} `nbt:"EnclosingSize"`
	} `nbt:"Metadata"`

	Regions map[string]regionNBT `nbt:"Regions"`

	Extra map[string]any `nbt:"*"`
}

type regionNBT struct {
	Position struct {
		X int32 `nbt:"x"`
		Y int32 `nbt:"y"`
		Z int32 `nbt:"z"`
	} `nbt:"Position"`

	Size struct {
		X int32 `nbt:"x"`
		Y int32 `nbt:"y"`
		Z int32 `nbt:"z"`
	} `nbt:"Size"`

	BlockStatePalette []struct {
		Name       string         `nbt:"Name"`
		Properties map[string]any `nbt:"Properties,omitempty"`
	} `nbt:"BlockStatePalette"`

	BlockStates       []int64          `nbt:"BlockStates,array"`
	TileEntities      []map[string]any `nbt:"TileEntities"`
	Entities          []map[string]any `nbt:"Entities"`
	PendingBlockTicks []map[string]any `nbt:"PendingBlockTicks,omitempty"`
	PendingFluidTicks []map[string]any `nbt:"PendingFluidTicks,omitempty"`
}

// Read reads a Litematica file (supports single region only).
func Read(r io.Reader) (base.Schematic, error) {
	// Decompress gzip
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip decompress: %w", err)
	}
	defer gz.Close()

	// Decode NBT
	var data litematicaNBT
	if err := nbt.NewDecoderWithEncoding(gz, nbt.BigEndian).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode nbt: %w", err)
	}

	if data.Version != 6 && data.Version != 7 {
		return nil, fmt.Errorf("unsupported litematica version: %d (expected 6 or 7)", data.Version)
	}

	// Find the first region
	var regionName string
	var regionData regionNBT
	for name, reg := range data.Regions {
		regionName = name
		regionData = reg
		break
	}

	if regionName == "" {
		return nil, fmt.Errorf("no regions found in litematica file")
	}

	// Build palette first
	palette := make([]*base.BlockState, len(regionData.BlockStatePalette))
	for i, p := range regionData.BlockStatePalette {
		palette[i] = &base.BlockState{
			Name:       p.Name,
			Properties: p.Properties,
		}
	}

	// Use absolute values for initial dimensions
	regWidth := int(math.Abs(float64(regionData.Size.X)))
	regHeight := int(math.Abs(float64(regionData.Size.Y)))
	regLength := int(math.Abs(float64(regionData.Size.Z)))

	// Decode blocks using tight packing
	bitsPerEntry := max(bits.Len(uint(len(palette))), 2)
	blockCount := regWidth * regHeight * regLength
	blockIndices := base.UnpackLongArrayTight(regionData.BlockStates, bitsPerEntry, blockCount)

	// Calculate actual bounding box from non-air blocks (like Axiom does)
	type blockPlacement struct {
		X, Y, Z int
		Block   *base.BlockState
	}
	placements := make([]blockPlacement, 0)
	minX, minY, minZ := math.MaxInt32, math.MaxInt32, math.MaxInt32
	maxX, maxY, maxZ := math.MinInt32, math.MinInt32, math.MinInt32
	hasContent := false

	for y := range regHeight {
		for z := range regLength {
			for x := range regWidth {
				idx := x + z*regWidth + y*regWidth*regLength
				if idx >= len(blockIndices) {
					continue
				}
				paletteIdx := blockIndices[idx]
				if paletteIdx < 0 || paletteIdx >= len(palette) {
					continue
				}
				block := palette[paletteIdx]
				if block == nil || isAirBlock(block.Name) {
					continue
				}

				placements = append(placements, blockPlacement{X: x, Y: y, Z: z, Block: block.Clone()})
				minX = min(minX, x)
				minY = min(minY, y)
				minZ = min(minZ, z)
				maxX = max(maxX, x)
				maxY = max(maxY, y)
				maxZ = max(maxZ, z)
				hasContent = true
			}
		}
	}

	// Calculate dimensions from bounding box
	var width, height, length int
	if hasContent {
		width = maxX - minX + 1
		height = maxY - minY + 1
		length = maxZ - minZ + 1
	} else {
		width = regWidth
		height = regHeight
		length = regLength
		minX, minY, minZ = 0, 0, 0
	}

	// Create schematic with calculated dimensions
	s := base.New(width, height, length, "litematica")
	s.SetDataVersion(int(data.MinecraftDataVersion))
	s.SetMetadata("Name", data.Metadata.Name)
	s.SetMetadata("Author", data.Metadata.Author)
	s.SetMetadata("Description", data.Metadata.Description)
	s.SetMetadata("RegionName", regionName)
	s.SetMetadata("TimeCreated", data.Metadata.TimeCreated)
	s.SetMetadata("TimeModified", data.Metadata.TimeModified)

	// Set offset (original position + bounding box offset)
	s.SetOffset(
		int(regionData.Position.X)+minX,
		int(regionData.Position.Y)+minY,
		int(regionData.Position.Z)+minZ,
	)

	// Set blocks using calculated offset
	for _, p := range placements {
		x := p.X - minX
		y := p.Y - minY
		z := p.Z - minZ
		s.SetBlock(x, y, z, p.Block)
	}

	// Set tile entities (adjust for offset)
	for _, teData := range regionData.TileEntities {
		be := &base.BlockEntity{
			Data: make(map[string]any),
		}

		// Extract position and adjust for bounding box
		var x, y, z int
		if xVal, ok := teData["x"].(int32); ok {
			x = int(xVal) - minX
		}
		if yVal, ok := teData["y"].(int32); ok {
			y = int(yVal) - minY
		}
		if zVal, ok := teData["z"].(int32); ok {
			z = int(zVal) - minZ
		}

		// Extract ID
		if id, ok := teData["id"].(string); ok {
			be.ID = id
		}

		// Copy remaining data
		for k, v := range teData {
			if k != "x" && k != "y" && k != "z" && k != "id" {
				be.Data[k] = v
			}
		}

		s.SetBlockEntity(x, y, z, be)
	}

	// Set entities (adjust for offset)
	for _, entData := range regionData.Entities {
		ent := &base.Entity{
			Data: make(map[string]any),
		}

		// Extract position and adjust for bounding box
		if pos, ok := entData["Pos"].([]any); ok && len(pos) >= 3 {
			ent.Pos[0] = pos[0].(float64) - float64(minX)
			ent.Pos[1] = pos[1].(float64) - float64(minY)
			ent.Pos[2] = pos[2].(float64) - float64(minZ)
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
		if id, ok := entData["id"].(string); ok {
			ent.ID = id
		}

		// Copy remaining data
		for k, v := range entData {
			if k != "Pos" && k != "Rotation" && k != "Motion" && k != "id" {
				ent.Data[k] = v
			}
		}

		s.AddEntity(ent)
	}

	return s, nil
}

// isAirBlock checks if a block name is an air variant.
func isAirBlock(name string) bool {
	switch name {
	case "", "minecraft:air", "minecraft:void_air", "minecraft:cave_air":
		return true
	default:
		return false
	}
}

// Write writes a schematic as Litematica (single region).
func Write(w io.Writer, schem base.Schematic) error {
	width, height, length := schem.Dimensions()
	offsetX, offsetY, offsetZ := schem.Offset()

	// Build palette
	palette := base.NewPaletteWithAir()
	blockIndices := make([]int, width*height*length)

	for y := range height {
		for z := range length {
			for x := range width {
				idx := x + z*width + y*width*length
				block := schem.Block(x, y, z)
				if block == nil {
					blockIndices[idx] = 0
				} else {
					blockIndices[idx] = palette.Add(*block)
				}
			}
		}
	}

	// Pack blocks using tight packing
	bitsPerEntry := max(bits.Len(uint(palette.Size())), 2)
	packedBlocks := base.PackLongArrayTight(blockIndices, bitsPerEntry)

	// Build region
	region := regionNBT{
		Position: struct {
			X int32 `nbt:"x"`
			Y int32 `nbt:"y"`
			Z int32 `nbt:"z"`
		}{X: int32(offsetX), Y: int32(offsetY), Z: int32(offsetZ)},
		Size: struct {
			X int32 `nbt:"x"`
			Y int32 `nbt:"y"`
			Z int32 `nbt:"z"`
		}{X: int32(width), Y: int32(height), Z: int32(length)},
		BlockStates: packedBlocks,
	}

	// Encode palette
	region.BlockStatePalette = make([]struct {
		Name       string         `nbt:"Name"`
		Properties map[string]any `nbt:"Properties,omitempty"`
	}, palette.Size())

	for i, block := range palette.Blocks() {
		region.BlockStatePalette[i].Name = block.Name
		region.BlockStatePalette[i].Properties = block.Properties
	}

	// Encode tile entities
	for y := range height {
		for z := range length {
			for x := range width {
				be := schem.BlockEntity(x, y, z)
				if be == nil {
					continue
				}

				teData := make(map[string]any)
				teData["x"] = int32(x)
				teData["y"] = int32(y)
				teData["z"] = int32(z)
				teData["id"] = be.ID
				maps.Copy(teData, be.Data)
				region.TileEntities = append(region.TileEntities, teData)
			}
		}
	}

	// Encode entities
	for _, ent := range schem.Entities() {
		entData := make(map[string]any)
		entData["Pos"] = []float64{ent.Pos[0], ent.Pos[1], ent.Pos[2]}
		entData["Rotation"] = []float32{ent.Rotation[0], ent.Rotation[1]}
		entData["Motion"] = []float64{ent.Motion[0], ent.Motion[1], ent.Motion[2]}
		entData["id"] = ent.ID
		maps.Copy(entData, ent.Data)
		region.Entities = append(region.Entities, entData)
	}

	// Build main structure
	meta := schem.Metadata()
	data := litematicaNBT{
		Version:              6,
		MinecraftDataVersion: int32(schem.DataVersion()),
		Regions:              map[string]regionNBT{"Region": region},
	}

	if name, ok := meta["Name"].(string); ok {
		data.Metadata.Name = name
	}
	if author, ok := meta["Author"].(string); ok {
		data.Metadata.Author = author
	}
	if desc, ok := meta["Description"].(string); ok {
		data.Metadata.Description = desc
	}
	if timeCreated, ok := meta["TimeCreated"].(int64); ok {
		data.Metadata.TimeCreated = timeCreated
	}
	if timeModified, ok := meta["TimeModified"].(int64); ok {
		data.Metadata.TimeModified = timeModified
	}

	data.Metadata.RegionCount = 1
	data.Metadata.TotalVolume = int32(width * height * length)
	data.Metadata.EnclosingSize.X = int32(width)
	data.Metadata.EnclosingSize.Y = int32(height)
	data.Metadata.EnclosingSize.Z = int32(length)

	// Count non-air blocks
	totalBlocks := 0
	for _, idx := range blockIndices {
		if idx > 0 {
			totalBlocks++
		}
	}
	data.Metadata.TotalBlocks = int32(totalBlocks)

	// Compress and write
	gz := gzip.NewWriter(w)
	if err := nbt.NewEncoderWithEncoding(gz, nbt.BigEndian).Encode(data); err != nil {
		gz.Close()
		return fmt.Errorf("encode nbt: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("close gzip: %w", err)
	}

	return nil
}
