package schem

import (
	_ "unsafe"

	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/oriumgames/crocon"
	"github.com/oriumgames/schem/format"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// Structure wraps a format.Schematic and implements world.Structure.
// It can be placed in a Dragonfly world using world.BuildStructure.
type Structure struct {
	schematic format.Schematic
	converter *crocon.Converter
}

// NewStructure creates a new Structure from a format.Schematic.
func NewStructure(s format.Schematic) *Structure {
	c, _ := crocon.NewConverter()
	return &Structure{
		schematic: s,
		converter: c,
	}
}

// Dimensions implements world.Structure.
func (s *Structure) Dimensions() [3]int {
	w, h, l := s.schematic.Dimensions()
	return [3]int{w, h, l}
}

// At implements world.Structure.
// It converts format.BlockState to world.Block using the crocon conversion system.
func (s *Structure) At(x, y, z int, _ func(x, y, z int) world.Block) (world.Block, world.Liquid) {
	state := s.schematic.Block(x, y, z)
	if state == nil {
		// Return air for nil blocks
		return block.Air{}, nil
	}

	// Special case: air should explicitly set air
	if state.Name == "minecraft:air" || state.Name == "air" {
		return block.Air{}, nil
	}

	// Determine source version from data version
	fromVersion := s.schematic.Version()
	if fromVersion == "" {
		return block.Air{}, nil
	}

	// Convert Java block to Bedrock
	b, err := s.converter.ConvertBlock(crocon.BlockRequest{
		ConversionRequest: crocon.ConversionRequest{
			FromVersion: fromVersion,
			ToVersion:   protocol.CurrentVersion,
			FromEdition: crocon.JavaEdition,
			ToEdition:   crocon.BedrockEdition,
		},
		Block: crocon.Block{
			ID:     state.Name,
			States: state.Properties,
		},
	})
	if err != nil {
		// Failed to convert - return air to skip
		return block.Air{}, nil
	}

	// Filter invalid properties
	validProps := blockProperties[b.ID]
	for k := range b.States {
		if _, ok := validProps[k]; !ok {
			delete(b.States, k)
		}
	}

	// Get the Bedrock block
	ret, ok := world.BlockByName(b.ID, b.States)
	if !ok {
		// Failed to convert - return air to skip
		return block.Air{}, nil
	}

	// Handle block entity data if present
	if nbter, ok := ret.(world.NBTer); ok {
		ent := s.schematic.BlockEntity(x, y, z)

		if ent != nil {
			from := crocon.BlockEntity(ent.Data)
			from["id"] = ent.ID

			be, err := s.converter.ConvertBlockEntity(crocon.BlockEntityRequest{
				ConversionRequest: crocon.ConversionRequest{
					FromVersion: fromVersion,
					ToVersion:   protocol.CurrentVersion,
					FromEdition: crocon.JavaEdition,
					ToEdition:   crocon.BedrockEdition,
				},
				BlockEntity: from,
			})
			if err != nil {
				// Failed to convert - return air to skip
				return block.Air{}, nil
			}

			m, ok := any(be).(*map[string]any)
			if !ok || m == nil {
				return block.Air{}, nil
			}

			tag, ok := (*m)["tag"].(map[string]any)
			if !ok {
				return block.Air{}, nil
			}

			return nbter.DecodeNBT(tag).(world.Block), nil
		} else {
			ret = nbter.DecodeNBT(map[string]any{}).(world.Block)
		}
	}

	// Handle waterlogged blocks
	var liquid world.Liquid
	if waterlogged, ok := state.Properties["waterlogged"].(bool); ok && waterlogged {
		liquid = block.Water{}
	}

	return ret, liquid
}

// Schematic returns the underlying format.Schematic.
func (s *Structure) Schematic() format.Schematic {
	return s.schematic
}

// Offset returns the structure's offset.
func (s *Structure) Offset() (x, y, z int) {
	return s.schematic.Offset()
}

// blockProperties is linked from dragonfly to validate block properties.
//
//go:linkname blockProperties github.com/df-mc/dragonfly/server/world.blockProperties
var blockProperties map[string]map[string]any
