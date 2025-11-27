package base

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Palette manages a mapping between block states and palette indices.
type Palette struct {
	blocks []BlockState
	index  map[string]int
}

// NewPalette creates a new empty palette.
func NewPalette() *Palette {
	return &Palette{
		blocks: make([]BlockState, 0),
		index:  make(map[string]int),
	}
}

// NewPaletteWithAir creates a new palette with air at index 0.
func NewPaletteWithAir() *Palette {
	p := NewPalette()
	p.Add(BlockState{Name: "minecraft:air"})
	return p
}

// Add adds a block state to the palette and returns its index.
// If the block state already exists, returns the existing index.
func (p *Palette) Add(block BlockState) int {
	key := blockStateKey(&block)
	if idx, ok := p.index[key]; ok {
		return idx
	}
	idx := len(p.blocks)
	p.blocks = append(p.blocks, block)
	p.index[key] = idx
	return idx
}

// Get returns the block state at the given index.
func (p *Palette) Get(idx int) *BlockState {
	if idx < 0 || idx >= len(p.blocks) {
		return nil
	}
	return &p.blocks[idx]
}

// Index returns the index of a block state, or -1 if not found.
func (p *Palette) Index(block BlockState) int {
	key := blockStateKey(&block)
	if idx, ok := p.index[key]; ok {
		return idx
	}
	return -1
}

// Size returns the number of entries in the palette.
func (p *Palette) Size() int {
	return len(p.blocks)
}

// Blocks returns all block states in the palette.
func (p *Palette) Blocks() []BlockState {
	return p.blocks
}

// blockStateKey generates a unique key for a block state.
func blockStateKey(block *BlockState) string {
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

	var buf strings.Builder
	buf.WriteString(block.Name)
	buf.WriteByte('[')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(k)
		buf.WriteByte('=')
		switch v := block.Properties[k].(type) {
		case string:
			buf.WriteString(v)
		case int, int32, int64:
			buf.WriteString(fmt.Sprint(v))
		case bool:
			buf.WriteString(strconv.FormatBool(v))
		default:
			buf.WriteString(fmt.Sprint(v))
		}
	}
	buf.WriteByte(']')
	return buf.String()
}
