# Schem

Universal minecraft schematics library 

## Key Features
- Multi-format support: Sponge (v1/v2/v3), Litematica, Axiom, MCEdit
- Auto-detection of schematic format
- Unified schematic interface across all formats
- Dragonfly integration: implements `world.Structure` interface
- Java to Bedrock block conversion via crocon
- Standalone format submodule (no Dragonfly dependency)

## Installation
Use Go modules:
- `go get github.com/oriumgames/schem`

## Quick Start
```go
// Read a schematic (auto-detects format)
structure, err := schem.ReadFile("build.litematic")
if err != nil {
    log.Fatal(err)
}

// Place in world
world.Exec(func(tx *world.Tx) {
		tx.BuildStructure(pos, structure)
})

// Access underlying format
schematic := structure.Schematic()
width, height, length := schematic.Dimensions()
```

## Supported Formats
- **Sponge Schematic v1/v2/v3** — `.schem` files, supports biomes and entities
- **Litematica** — `.litematic` files, supports single-region schematics
- **Axiom** — `.axiom` files, chunk-based storage with thumbnails
- **MCEdit** — `.schematic` files, legacy format with block ID/metadata

## Format Submodule
The `format` package can be used standalone without Dragonfly dependencies:

```go
import "github.com/oriumgames/schem/format"

// Read schematic
schematic, err := format.ReadFile("build.schem")

// Access data
block := schematic.Block(x, y, z)
entity := schematic.BlockEntity(x, y, z)
biome := schematic.Biome(x, y, z)

// Write to different format
format.WriteFormat(writer, "litematica", schematic)
```

## API Reference

### Main Package (schem)
- `Read(r io.Reader) (*Structure, error)` — Read with auto-detection
- `ReadFile(path string) (*Structure, error)` — Read from file
- `ReadFormat(r io.Reader, formatID string) (*Structure, error)` — Read specific format
- `Write(w io.Writer, s *Structure) error` — Write in native format
- `WriteFile(path string, s *Structure) error` — Write to file
- `WriteFormat(w io.Writer, formatID string, s *Structure) error` — Write specific format
- `Formats() []string` — List supported format IDs

### Format Package (format)
- `Detect(data []byte) (string, error)` — Auto-detect format
- `Read(r io.Reader) (Schematic, error)` — Read with auto-detection
- `ReadFormat(r io.Reader, formatID string) (Schematic, error)` — Read specific format
- `Write(w io.Writer, schem Schematic) error` — Write in native format
- `WriteFormat(w io.Writer, formatID string, schem Schematic) error` — Write specific format

### Schematic Interface
```go
type Schematic interface {
    Dimensions() (width, height, length int)
    Offset() (x, y, z int)
    SetOffset(x, y, z int)
    
    Block(x, y, z int) *BlockState
    SetBlock(x, y, z int, block *BlockState)
    
    BlockEntity(x, y, z int) *BlockEntity
    SetBlockEntity(x, y, z int, be *BlockEntity)
    
    Entities() []*Entity
    AddEntity(entity *Entity)
    RemoveEntity(entity *Entity)
    
    Biome(x, y, z int) string
    SetBiome(x, y, z int, biome string)
    
    Metadata() map[string]any
    SetMetadata(key string, value any)
    
    Format() string
    DataVersion() int
    SetDataVersion(version int)
}
```

## Format Detection
Format detection is automatic based on file structure:
- **Axiom**: Binary magic number `0x0AE5BB36`
- **Litematica**: Gzip + NBT with `Version` (6/7) and `Regions` tag
- **Sponge**: Gzip + NBT with `Version` tag (1/2/3)
- **MCEdit**: Gzip + NBT with `Materials`, `Blocks`, `Data` tags

## Conversion Details
When placing in Dragonfly worlds:
- Java block states are converted to Bedrock using crocon
- Invalid properties are filtered based on Dragonfly's block registry
- Block entity NBT data is preserved and applied
- Air blocks are handled explicitly to clear existing blocks
- Unsupported blocks default to air

## Notes & Limits
- Litematica: Single-region only (multi-region files use first region)
- Biomes: Support varies by format (v1=none, v2=2D, v3=3D)
- Dimension validation: All formats reject invalid/negative dimensions
- Bounding box optimization: Litematica and Axiom calculate tight bounds
- Block entities and entities are adjusted for calculated offsets

## Examples
```go
// Convert between formats
litematic, _ := format.Read(litematicaFile)
format.WriteFormat(spongeFile, "sponge_v3", litematic)

// Inspect schematic
schematic, _ := format.ReadFile("build.schem")
fmt.Printf("Size: %dx%dx%d\n", schematic.Dimensions())
fmt.Printf("Format: %s\n", schematic.Format())
fmt.Printf("Data Version: %d\n", schematic.DataVersion())

// Modify and save
schematic.SetBlock(0, 0, 0, &format.BlockState{
    Name: "minecraft:stone",
})
format.WriteFile(output, schematic)
```
