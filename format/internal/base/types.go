package base

import (
	"fmt"
	"maps"
	"sort"
	"strconv"
	"strings"
)

// BlockState represents a block with its properties.
type BlockState struct {
	Name       string         // e.g., "minecraft:oak_stairs"
	Properties map[string]any // e.g., {"facing": "north", "half": "bottom"}
}

// Clone creates a deep copy of the BlockState.
func (b *BlockState) Clone() *BlockState {
	if b == nil {
		return nil
	}
	props := make(map[string]any, len(b.Properties))
	maps.Copy(props, b.Properties)
	return &BlockState{
		Name:       b.Name,
		Properties: props,
	}
}

// String returns a string representation of the block state.
func (b *BlockState) String() string {
	if len(b.Properties) == 0 {
		return b.Name
	}
	keys := make([]string, 0, len(b.Properties))
	for k := range b.Properties {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, b.Properties[k]))
	}
	return b.Name + "[" + strings.Join(parts, ",") + "]"
}

// ParseBlockState parses a block state string into a BlockState.
func ParseBlockState(s string) *BlockState {
	name, props, _ := strings.Cut(s, "[")
	if props == "" {
		return &BlockState{Name: name}
	}

	props = strings.TrimSuffix(props, "]")
	properties := make(map[string]any)

	for part := range strings.SplitSeq(props, ",") {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}

		if value == "true" {
			properties[key] = true
		} else if value == "false" {
			properties[key] = false
		} else if i, err := strconv.Atoi(value); err == nil {
			properties[key] = int32(i)
		} else {
			properties[key] = value
		}
	}

	return &BlockState{
		Name:       name,
		Properties: properties,
	}
}

// BlockEntity represents a block entity (tile entity).
type BlockEntity struct {
	ID      string         // e.g., "minecraft:chest"
	X, Y, Z int            // Position relative to schematic origin
	Data    map[string]any // NBT data (excluding position and id)
}

// Clone creates a deep copy of the BlockEntity.
func (be *BlockEntity) Clone() *BlockEntity {
	if be == nil {
		return nil
	}
	data := make(map[string]any, len(be.Data))
	for k, v := range be.Data {
		data[k] = deepCopy(v)
	}
	return &BlockEntity{
		ID:   be.ID,
		X:    be.X,
		Y:    be.Y,
		Z:    be.Z,
		Data: data,
	}
}

// Entity represents a movable entity.
type Entity struct {
	ID       string         // e.g., "minecraft:armor_stand"
	Pos      [3]float64     // Position (x, y, z) relative to schematic origin
	Rotation [2]float32     // Rotation (yaw, pitch) in degrees
	Motion   [3]float64     // Velocity (x, y, z)
	UUID     *[4]int32      // Optional UUID
	Data     map[string]any // NBT data (excluding Pos, Rotation, Motion, UUID, id)
}

// Clone creates a deep copy of the Entity.
func (e *Entity) Clone() *Entity {
	if e == nil {
		return nil
	}
	data := make(map[string]any, len(e.Data))
	for k, v := range e.Data {
		data[k] = deepCopy(v)
	}
	entity := &Entity{
		ID:       e.ID,
		Pos:      e.Pos,
		Rotation: e.Rotation,
		Motion:   e.Motion,
		Data:     data,
	}
	if e.UUID != nil {
		uuid := *e.UUID
		entity.UUID = &uuid
	}
	return entity
}

// deepCopy performs a deep copy of interface{} values.
func deepCopy(v any) any {
	switch val := v.(type) {
	case map[string]any:
		copy := make(map[string]any, len(val))
		for k, v := range val {
			copy[k] = deepCopy(v)
		}
		return copy
	case []any:
		copy := make([]any, len(val))
		for i, v := range val {
			copy[i] = deepCopy(v)
		}
		return copy
	case []byte:
		b := make([]byte, len(val))
		copy(b, val)
		return b
	default:
		return v
	}
}

// Schematic is the universal interface for all schematic formats.
type Schematic interface {
	// Dimensions returns the dimensions of the schematic in blocks (width, height, length).
	Dimensions() (width, height, length int)

	// Offset returns the origin offset of the schematic.
	Offset() (x, y, z int)

	// SetOffset sets the origin offset.
	SetOffset(x, y, z int)

	// Block returns the block state at the given position.
	// Returns nil if the position is out of bounds or is air/empty.
	Block(x, y, z int) *BlockState

	// SetBlock sets a block at the given position.
	// Pass nil to clear the block (set to air).
	SetBlock(x, y, z int, block *BlockState)

	// BlockEntity returns the block entity at the given position.
	// Returns nil if no block entity exists at that position.
	BlockEntity(x, y, z int) *BlockEntity

	// SetBlockEntity sets a block entity at the given position.
	// Pass nil to remove the block entity.
	SetBlockEntity(x, y, z int, be *BlockEntity)

	// Entities returns all entities in the schematic.
	Entities() []*Entity

	// AddEntity adds an entity to the schematic.
	AddEntity(entity *Entity)

	// RemoveEntity removes an entity from the schematic.
	RemoveEntity(entity *Entity)

	// Biome returns the biome at the given position.
	// Returns empty string if biomes are not supported or not set.
	Biome(x, y, z int) string

	// SetBiome sets the biome at the given position.
	SetBiome(x, y, z int, biome string)

	// Metadata returns format-specific metadata.
	Metadata() map[string]any

	// SetMetadata sets a metadata key-value pair.
	SetMetadata(key string, value any)

	// Format returns the format identifier (e.g., "sponge_v3", "litematica").
	Format() string

	// DataVersion returns the Minecraft data version (e.g., 3465 for 1.20.1).
	// Returns 0 if not applicable.
	DataVersion() int

	// SetDataVersion sets the Minecraft data version.
	SetDataVersion(version int)

	// Version returns the Minecraft version string (e.g., 1.20.1).
	// Returns "" if not applicable.
	Version() string
}
