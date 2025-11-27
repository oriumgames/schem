package axiom

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"maps"
	"math"
	"reflect"
	"sort"
	"strings"

	"github.com/oriumgames/nbt"
	"github.com/oriumgames/pile/schem/format/internal/base"
)

const (
	Magic             uint32 = 0x0AE5BB36
	chunkSize         int32  = 16
	chunkArea                = chunkSize * chunkSize
	chunkVolume              = chunkSize * chunkArea
	defaultEmptyBlock        = "minecraft:structure_void"
)

type headerNBT struct {
	Version         int32          `nbt:"Version"`
	Name            string         `nbt:"Name,omitempty"`
	Author          string         `nbt:"Author,omitempty"`
	Tags            []string       `nbt:"Tags,omitempty"`
	ThumbnailYaw    float32        `nbt:"ThumbnailYaw,omitempty"`
	ThumbnailPitch  float32        `nbt:"ThumbnailPitch,omitempty"`
	LockedThumbnail bool           `nbt:"LockedThumbnail,omitempty"`
	BlockCount      int32          `nbt:"BlockCount,omitempty"`
	ContainsAir     bool           `nbt:"ContainsAir,omitempty"`
	Extra           map[string]any `nbt:"*"`
}

type blockDataNBT struct {
	DataVersion   int32            `nbt:"DataVersion"`
	BlockRegion   []chunkNBT       `nbt:"BlockRegion"`
	BlockEntities []map[string]any `nbt:"BlockEntities,omitempty"`
	Entities      []map[string]any `nbt:"Entities,omitempty"`
	Extra         map[string]any   `nbt:"*"`
}

type chunkNBT struct {
	X                 int32               `nbt:"X"`
	Y                 int32               `nbt:"Y"`
	Z                 int32               `nbt:"Z"`
	BlockStates       chunkBlockStatesNBT `nbt:"BlockStates,omitempty"`
	LegacyBlockStates chunkBlockStatesNBT `nbt:"data,omitempty"`
}

type chunkBlockStatesNBT struct {
	Palette []paletteEntryNBT `nbt:"palette"`
	Data    any               `nbt:"data"`
}

func (c chunkNBT) states() chunkBlockStatesNBT {
	if len(c.BlockStates.Palette) > 0 || c.BlockStates.Data != nil {
		return c.BlockStates
	}
	return c.LegacyBlockStates
}

func (b chunkBlockStatesNBT) longs() ([]int64, error) {
	if b.Data == nil {
		return nil, nil
	}
	if longs, ok := b.Data.([]int64); ok {
		return longs, nil
	}
	rv := reflect.ValueOf(b.Data)
	switch rv.Kind() {
	case reflect.Array, reflect.Slice:
		out := make([]int64, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			val := rv.Index(i).Interface()
			n, ok := toInt64(val)
			if !ok {
				return nil, fmt.Errorf("unexpected element type %T", val)
			}
			out[i] = n
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unexpected block state data type %T", b.Data)
	}
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int32:
		return int64(n), true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}

type paletteEntryNBT struct {
	Name       string         `nbt:"Name"`
	Properties map[string]any `nbt:"Properties,omitempty"`
}

type blockPlacement struct {
	X, Y, Z int32
	Block   *base.BlockState
}

type chunkKey struct {
	X, Y, Z int32
}

type chunkBuilder struct {
	palette map[string]int32
	entries []paletteEntryNBT
	data    []int
}

func newChunkBuilder() *chunkBuilder {
	entries := []paletteEntryNBT{{Name: defaultEmptyBlock}}
	palette := map[string]int32{defaultEmptyBlock: 0}
	return &chunkBuilder{
		palette: palette,
		entries: entries,
		data:    make([]int, int(chunkVolume)),
	}
}

func (cb *chunkBuilder) paletteIndex(block *base.BlockState) int32 {
	if block == nil {
		return 0
	}
	key := blockStateKey(block)
	if idx, ok := cb.palette[key]; ok {
		return idx
	}
	entry := paletteEntryNBT{Name: block.Name}
	if len(block.Properties) > 0 {
		entry.Properties = block.Properties
	}
	idx := len(cb.entries)
	cb.entries = append(cb.entries, entry)
	cb.palette[key] = int32(idx)
	return int32(idx)
}

func (cb *chunkBuilder) set(localX, localY, localZ int32, block *base.BlockState) {
	idx := int(localY*chunkArea + localZ*chunkSize + localX)
	cb.data[idx] = int(cb.paletteIndex(block))
}

func (cb *chunkBuilder) toNBT(x, y, z int32) chunkNBT {
	entries := make([]paletteEntryNBT, len(cb.entries))
	copy(entries, cb.entries)
	bits := bitsPerBlock(len(entries))
	data := base.PackLongArray(cb.data, bits)
	return chunkNBT{
		X: x,
		Y: y,
		Z: z,
		BlockStates: chunkBlockStatesNBT{
			Palette: entries,
			Data:    data,
		},
	}
}

// Read reads an Axiom blueprint file.
func Read(r io.Reader) (base.Schematic, error) {
	var magic uint32
	if err := binary.Read(r, binary.BigEndian, &magic); err != nil {
		return nil, fmt.Errorf("read magic: %w", err)
	}
	if magic != Magic {
		return nil, fmt.Errorf("invalid magic: expected 0x%X, got 0x%X", Magic, magic)
	}

	var headerLen uint32
	if err := binary.Read(r, binary.BigEndian, &headerLen); err != nil {
		return nil, fmt.Errorf("read header length: %w", err)
	}
	headerBuf := make([]byte, headerLen)
	if _, err := io.ReadFull(r, headerBuf); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	var header headerNBT
	if err := nbt.NewDecoderWithEncoding(bytes.NewReader(headerBuf), nbt.BigEndian).Decode(&header); err != nil {
		return nil, fmt.Errorf("decode header nbt: %w", err)
	}

	var thumbLen uint32
	if err := binary.Read(r, binary.BigEndian, &thumbLen); err != nil {
		return nil, fmt.Errorf("read thumbnail length: %w", err)
	}
	thumbnail := make([]byte, thumbLen)
	if thumbLen > 0 {
		if _, err := io.ReadFull(r, thumbnail); err != nil {
			return nil, fmt.Errorf("read thumbnail: %w", err)
		}
	}

	var dataLen uint32
	if err := binary.Read(r, binary.BigEndian, &dataLen); err != nil {
		return nil, fmt.Errorf("read data length: %w", err)
	}
	dataBuf := make([]byte, dataLen)
	if _, err := io.ReadFull(r, dataBuf); err != nil {
		return nil, fmt.Errorf("read data: %w", err)
	}

	gz, err := gzip.NewReader(bytes.NewReader(dataBuf))
	if err != nil {
		return nil, fmt.Errorf("gzip decompress: %w", err)
	}
	defer gz.Close()

	var blockData blockDataNBT
	if err := nbt.NewDecoderWithEncoding(gz, nbt.BigEndian).Decode(&blockData); err != nil {
		return nil, fmt.Errorf("decode block data nbt: %w", err)
	}

	placements := make([]blockPlacement, 0)
	minX, minY, minZ := math.MaxInt32, math.MaxInt32, math.MaxInt32
	maxX, maxY, maxZ := math.MinInt32, math.MinInt32, math.MinInt32
	hasContent := false
	blockCount := 0

	for _, chunk := range blockData.BlockRegion {
		states := chunk.states()
		palette := make([]*base.BlockState, len(states.Palette))
		for i, entry := range states.Palette {
			block := &base.BlockState{Name: entry.Name}
			if len(entry.Properties) > 0 {
				block.Properties = entry.Properties
			}
			palette[i] = block
		}

		longs, err := states.longs()
		if err != nil {
			return nil, fmt.Errorf("chunk %d,%d,%d block data: %w", chunk.X, chunk.Y, chunk.Z, err)
		}
		bits := bitsPerBlock(len(palette))
		values := base.UnpackLongArray(longs, bits, int(chunkVolume))

		for idx, paletteIdx := range values {
			if paletteIdx < 0 || paletteIdx >= len(palette) {
				continue
			}
			block := palette[paletteIdx]
			if block == nil || isEmptyBlock(block.Name) {
				continue
			}

			localY := int32(idx) / chunkArea
			rem := int32(idx) % chunkArea
			localZ := rem / chunkSize
			localX := rem % chunkSize

			globalX := chunk.X*chunkSize + localX
			globalY := chunk.Y*chunkSize + localY
			globalZ := chunk.Z*chunkSize + localZ

			placements = append(placements, blockPlacement{X: globalX, Y: globalY, Z: globalZ, Block: block.Clone()})

			minX = min(minX, int(globalX))
			minY = min(minY, int(globalY))
			minZ = min(minZ, int(globalZ))
			maxX = max(maxX, int(globalX))
			maxY = max(maxY, int(globalY))
			maxZ = max(maxZ, int(globalZ))
			hasContent = true
			blockCount++
		}
	}

	rawBlockEntities := make([]*base.BlockEntity, 0, len(blockData.BlockEntities))
	for _, raw := range blockData.BlockEntities {
		x, okX := raw["x"].(int32)
		y, okY := raw["y"].(int32)
		z, okZ := raw["z"].(int32)
		if !okX || !okY || !okZ {
			continue
		}
		id, _ := raw["id"].(string)
		be := &base.BlockEntity{ID: id, X: int(x), Y: int(y), Z: int(z), Data: make(map[string]any)}
		for k, v := range raw {
			switch strings.ToLower(k) {
			case "x", "y", "z", "id":
				continue
			default:
				be.Data[k] = v
			}
		}
		rawBlockEntities = append(rawBlockEntities, be)
		minX = min(minX, int(x))
		minY = min(minY, int(y))
		minZ = min(minZ, int(z))
		maxX = max(maxX, int(x))
		maxY = max(maxY, int(y))
		maxZ = max(maxZ, int(z))
		hasContent = true
	}

	rawEntities := make([]*base.Entity, 0, len(blockData.Entities))
	for _, raw := range blockData.Entities {
		ent := &base.Entity{Data: make(map[string]any)}
		if id, ok := raw["id"].(string); ok {
			ent.ID = id
		}
		if pos, ok := raw["Pos"].([]float64); ok && len(pos) >= 3 {
			ent.Pos[0], ent.Pos[1], ent.Pos[2] = pos[0], pos[1], pos[2]
			minX = min(minX, int(math.Floor(pos[0])))
			minY = min(minY, int(math.Floor(pos[1])))
			minZ = min(minZ, int(math.Floor(pos[2])))
			maxX = max(maxX, int(math.Ceil(pos[0])))
			maxY = max(maxY, int(math.Ceil(pos[1])))
			maxZ = max(maxZ, int(math.Ceil(pos[2])))
			hasContent = true
		}
		if rot, ok := raw["Rotation"].([]float32); ok && len(rot) >= 2 {
			ent.Rotation[0], ent.Rotation[1] = rot[0], rot[1]
		}
		if motion, ok := raw["Motion"].([]float64); ok && len(motion) >= 3 {
			ent.Motion[0], ent.Motion[1], ent.Motion[2] = motion[0], motion[1], motion[2]
		}
		for k, v := range raw {
			if strings.EqualFold(k, "id") || strings.EqualFold(k, "pos") || strings.EqualFold(k, "position") || strings.EqualFold(k, "rotation") || strings.EqualFold(k, "motion") {
				continue
			}
			ent.Data[k] = v
		}
		rawEntities = append(rawEntities, ent)
	}

	if !hasContent {
		s := base.New(0, 0, 0, "axiom")
		s.SetDataVersion(int(blockData.DataVersion))
		recordHeaderMetadata(s, &header, 0, header.ContainsAir)
		return s, nil
	}

	width := maxX - minX + 1
	height := maxY - minY + 1
	length := maxZ - minZ + 1
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	if length < 1 {
		length = 1
	}

	s := base.New(width, height, length, "axiom")
	s.SetOffset(minX, minY, minZ)
	s.SetDataVersion(int(blockData.DataVersion))

	containsAirComputed := blockCount < width*height*length
	recordHeaderMetadata(s, &header, blockCount, containsAirComputed)

	for _, placement := range placements {
		x := int(placement.X) - minX
		y := int(placement.Y) - minY
		z := int(placement.Z) - minZ
		if x < 0 || x >= width || y < 0 || y >= height || z < 0 || z >= length {
			continue
		}
		s.SetBlock(x, y, z, placement.Block)
	}

	for _, be := range rawBlockEntities {
		lx := be.X - minX
		ly := be.Y - minY
		lz := be.Z - minZ
		if lx < 0 || lx >= width || ly < 0 || ly >= height || lz < 0 || lz >= length {
			continue
		}
		clone := be.Clone()
		clone.X, clone.Y, clone.Z = int(lx), int(ly), int(lz)
		s.SetBlockEntity(int(lx), int(ly), int(lz), clone)
	}

	for _, ent := range rawEntities {
		clone := ent.Clone()
		clone.Pos[0] -= float64(minX)
		clone.Pos[1] -= float64(minY)
		clone.Pos[2] -= float64(minZ)
		s.AddEntity(clone)
	}

	return s, nil
}

// Write writes a schematic as Axiom blueprint format.
func Write(w io.Writer, schem base.Schematic) error {
	width, height, length := schem.Dimensions()
	offsetX, offsetY, offsetZ := schem.Offset()

	header := &headerNBT{Version: 1}

	chunks := make(map[chunkKey]*chunkBuilder)
	blockCount := 0
	containsAir := false

	for y := range height {
		for z := range length {
			for x := range width {
				block := schem.Block(x, y, z)
				if block == nil || isEmptyBlock(block.Name) {
					containsAir = true
					continue
				}

				worldX := int32(x + offsetX)
				worldY := int32(y + offsetY)
				worldZ := int32(z + offsetZ)

				chunkX := worldX / chunkSize
				localX := worldX % chunkSize
				if localX < 0 {
					localX += chunkSize
					chunkX -= 1
				}
				chunkY := worldY / chunkSize
				localY := worldY % chunkSize
				if localY < 0 {
					localY += chunkSize
					chunkY -= 1
				}
				chunkZ := worldZ / chunkSize
				localZ := worldZ % chunkSize
				if localZ < 0 {
					localZ += chunkSize
					chunkZ -= 1
				}

				key := chunkKey{X: chunkX, Y: chunkY, Z: chunkZ}
				builder, ok := chunks[key]
				if !ok {
					builder = newChunkBuilder()
					chunks[key] = builder
				}
				builder.set(localX, localY, localZ, block)
				blockCount++
			}
		}
	}

	blockEntityMaps := collectBlockEntities(schem, offsetX, offsetY, offsetZ)
	entityMaps := collectEntities(schem, offsetX, offsetY, offsetZ)

	keys := make([]chunkKey, 0, len(chunks))
	for key := range chunks {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Y != keys[j].Y {
			return keys[i].Y < keys[j].Y
		}
		if keys[i].Z != keys[j].Z {
			return keys[i].Z < keys[j].Z
		}
		return keys[i].X < keys[j].X
	})

	chunkList := make([]chunkNBT, 0, len(keys))
	for _, key := range keys {
		chunkList = append(chunkList, chunks[key].toNBT(key.X, key.Y, key.Z))
	}
	if len(chunkList) == 0 {
		chunkList = append(chunkList, newChunkBuilder().toNBT(0, 0, 0))
	}

	if blockCount == 0 {
		containsAir = true
	}
	if header.Version == 0 {
		header.Version = 1
	}
	if header.Name == "" {
		header.Name = "Converted Blueprint"
	}
	if len(header.Tags) == 0 {
		header.Tags = []string{"converted"}
	}
	header.BlockCount = int32(blockCount)
	if !header.ContainsAir && containsAir {
		header.ContainsAir = true
	}

	blockData := blockDataNBT{
		DataVersion: int32(schem.DataVersion()),
		BlockRegion: chunkList,
	}
	if len(blockEntityMaps) > 0 {
		blockData.BlockEntities = blockEntityMaps
	}
	if len(entityMaps) > 0 {
		blockData.Entities = entityMaps
	}

	var headerBuf bytes.Buffer
	if err := nbt.NewEncoderWithEncoding(&headerBuf, nbt.BigEndian).Encode(header); err != nil {
		return fmt.Errorf("encode header nbt: %w", err)
	}
	if headerBuf.Len() > int(math.MaxUint32) {
		return fmt.Errorf("header too large: %d bytes", headerBuf.Len())
	}

	var dataBuf bytes.Buffer
	gz := gzip.NewWriter(&dataBuf)
	if err := nbt.NewEncoderWithEncoding(gz, nbt.BigEndian).Encode(blockData); err != nil {
		gz.Close()
		return fmt.Errorf("encode block data nbt: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("close gzip: %w", err)
	}
	if dataBuf.Len() > int(math.MaxUint32) {
		return fmt.Errorf("block data too large: %d bytes", dataBuf.Len())
	}

	if err := binary.Write(w, binary.BigEndian, Magic); err != nil {
		return fmt.Errorf("write magic: %w", err)
	}
	if err := binary.Write(w, binary.BigEndian, uint32(headerBuf.Len())); err != nil {
		return fmt.Errorf("write header length: %w", err)
	}
	if _, err := w.Write(headerBuf.Bytes()); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	if len(header.Tags) > int(math.MaxUint32) {
		return fmt.Errorf("thumbnail too large: %d bytes", len(header.Tags))
	}
	if err := binary.Write(w, binary.BigEndian, uint32(0)); err != nil {
		return fmt.Errorf("write thumbnail length: %w", err)
	}

	if err := binary.Write(w, binary.BigEndian, uint32(dataBuf.Len())); err != nil {
		return fmt.Errorf("write data length: %w", err)
	}
	if _, err := w.Write(dataBuf.Bytes()); err != nil {
		return fmt.Errorf("write block data: %w", err)
	}

	return nil
}

func bitsPerBlock(paletteSize int) int {
	if paletteSize <= 0 {
		return 0
	}
	size := paletteSize - 1
	bits := 0
	for size > 0 {
		size >>= 1
		bits++
	}
	if bits < 4 {
		bits = 4
	}
	return bits
}

func isEmptyBlock(name string) bool {
	switch name {
	case "", "minecraft:air", "minecraft:void_air", "minecraft:cave_air", "minecraft:structure_void":
		return true
	default:
		return false
	}
}

func blockStateKey(block *base.BlockState) string {
	if block == nil {
		return ""
	}
	if len(block.Properties) == 0 {
		return block.Name
	}
	keys := make([]string, 0, len(block.Properties))
	for k := range block.Properties {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var builder strings.Builder
	builder.WriteString(block.Name)
	builder.WriteByte('[')
	for i, key := range keys {
		if i > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(key)
		builder.WriteByte('=')
		builder.WriteString(fmt.Sprint(block.Properties[key]))
	}
	builder.WriteByte(']')
	return builder.String()
}

func recordHeaderMetadata(s base.Schematic, header *headerNBT, computedBlocks int, containsAir bool) {
	if header == nil {
		return
	}
	if header.Name != "" {
		s.SetMetadata("Name", header.Name)
	}
	if header.Author != "" {
		s.SetMetadata("Author", header.Author)
	}
	if len(header.Tags) > 0 {
		s.SetMetadata("Tags", append([]string(nil), header.Tags...))
	}
	s.SetMetadata("Version", header.Version)
	s.SetMetadata("BlockCount", int(header.BlockCount))
	s.SetMetadata("ContainsAir", header.ContainsAir)
	if header.ThumbnailYaw != 0 {
		s.SetMetadata("ThumbnailYaw", header.ThumbnailYaw)
	}
	if header.ThumbnailPitch != 0 {
		s.SetMetadata("ThumbnailPitch", header.ThumbnailPitch)
	}
	if header.LockedThumbnail {
		s.SetMetadata("LockedThumbnail", header.LockedThumbnail)
	}
	s.SetMetadata("ComputedBlockCount", computedBlocks)
	if containsAir {
		s.SetMetadata("ComputedContainsAir", true)
	}
}

func collectBlockEntities(schem base.Schematic, offsetX, offsetY, offsetZ int) []map[string]any {
	width, height, length := schem.Dimensions()
	result := make([]map[string]any, 0)
	for y := range height {
		for z := range length {
			for x := range width {
				be := schem.BlockEntity(x, y, z)
				if be == nil {
					continue
				}
				m := make(map[string]any, len(be.Data)+4)
				m["x"] = be.X + offsetX
				m["y"] = be.Y + offsetY
				m["z"] = be.Z + offsetZ
				if be.ID != "" {
					m["id"] = be.ID
				}
				maps.Copy(m, be.Data)
				result = append(result, m)
			}
		}
	}
	return result
}

func collectEntities(schem base.Schematic, offsetX, offsetY, offsetZ int) []map[string]any {
	entities := schem.Entities()
	result := make([]map[string]any, 0, len(entities))
	for _, ent := range entities {
		m := make(map[string]any, len(ent.Data)+4)
		m["Pos"] = []float64{
			ent.Pos[0] + float64(offsetX),
			ent.Pos[1] + float64(offsetY),
			ent.Pos[2] + float64(offsetZ),
		}
		m["Rotation"] = []float32{ent.Rotation[0], ent.Rotation[1]}
		m["Motion"] = []float64{ent.Motion[0], ent.Motion[1], ent.Motion[2]}
		if ent.ID != "" {
			m["id"] = ent.ID
		}
		if ent.UUID != nil {
			uuid := *ent.UUID
			m["UUID"] = []int32{uuid[0], uuid[1], uuid[2], uuid[3]}
		}
		maps.Copy(m, ent.Data)
		result = append(result, m)
	}
	return result
}
