package base

import "maps"

// SchematicImpl is a sparse storage implementation of Schematic interface.
// It stores only non-air blocks to minimize memory usage.
type SchematicImpl struct {
	width, height, length     int
	offsetX, offsetY, offsetZ int

	blocks        map[int]*BlockState
	blockEntities map[int]*BlockEntity
	biomes        map[int]string
	entities      []*Entity
	metadata      map[string]any
	formatID      string
	dataVersion   int
}

// New creates a new schematic with the given dimensions and format ID.
func New(width, height, length int, formatID string) *SchematicImpl {
	return &SchematicImpl{
		width:         width,
		height:        height,
		length:        length,
		blocks:        make(map[int]*BlockState),
		blockEntities: make(map[int]*BlockEntity),
		biomes:        make(map[int]string),
		entities:      make([]*Entity, 0),
		metadata:      make(map[string]any),
		formatID:      formatID,
	}
}

func (s *SchematicImpl) index(x, y, z int) int {
	return x + z*s.width + y*s.width*s.length
}

func (s *SchematicImpl) Dimensions() (int, int, int) {
	return s.width, s.height, s.length
}

func (s *SchematicImpl) Offset() (int, int, int) {
	return s.offsetX, s.offsetY, s.offsetZ
}

func (s *SchematicImpl) SetOffset(x, y, z int) {
	s.offsetX, s.offsetY, s.offsetZ = x, y, z
}

func (s *SchematicImpl) Block(x, y, z int) *BlockState {
	if x < 0 || x >= s.width || y < 0 || y >= s.height || z < 0 || z >= s.length {
		return nil
	}
	return s.blocks[s.index(x, y, z)]
}

func (s *SchematicImpl) SetBlock(x, y, z int, block *BlockState) {
	if x < 0 || x >= s.width || y < 0 || y >= s.height || z < 0 || z >= s.length {
		return
	}
	idx := s.index(x, y, z)
	if block == nil {
		delete(s.blocks, idx)
	} else {
		s.blocks[idx] = block
	}
}

func (s *SchematicImpl) BlockEntity(x, y, z int) *BlockEntity {
	if x < 0 || x >= s.width || y < 0 || y >= s.height || z < 0 || z >= s.length {
		return nil
	}
	return s.blockEntities[s.index(x, y, z)]
}

func (s *SchematicImpl) SetBlockEntity(x, y, z int, be *BlockEntity) {
	if x < 0 || x >= s.width || y < 0 || y >= s.height || z < 0 || z >= s.length {
		return
	}
	idx := s.index(x, y, z)
	if be == nil {
		delete(s.blockEntities, idx)
	} else {
		be.X, be.Y, be.Z = x, y, z
		s.blockEntities[idx] = be
	}
}

func (s *SchematicImpl) Entities() []*Entity {
	entities := make([]*Entity, len(s.entities))
	copy(entities, s.entities)
	return entities
}

func (s *SchematicImpl) AddEntity(entity *Entity) {
	s.entities = append(s.entities, entity)
}

func (s *SchematicImpl) RemoveEntity(entity *Entity) {
	for i, e := range s.entities {
		if e == entity {
			s.entities = append(s.entities[:i], s.entities[i+1:]...)
			return
		}
	}
}

func (s *SchematicImpl) Biome(x, y, z int) string {
	if x < 0 || x >= s.width || z < 0 || z >= s.length {
		return ""
	}
	// Try 3D biome first
	if y >= 0 && y < s.height {
		if biome, ok := s.biomes[s.index(x, y, z)]; ok {
			return biome
		}
	}
	// Fall back to 2D biome
	return s.biomes[x+z*s.width]
}

func (s *SchematicImpl) SetBiome(x, y, z int, biome string) {
	if x < 0 || x >= s.width || z < 0 || z >= s.length {
		return
	}
	var idx int
	if y >= 0 && y < s.height {
		idx = s.index(x, y, z)
	} else {
		idx = x + z*s.width
	}
	if biome == "" {
		delete(s.biomes, idx)
	} else {
		s.biomes[idx] = biome
	}
}

func (s *SchematicImpl) Metadata() map[string]any {
	m := make(map[string]any, len(s.metadata))
	maps.Copy(m, s.metadata)
	return m
}

func (s *SchematicImpl) SetMetadata(key string, value any) {
	s.metadata[key] = value
}

func (s *SchematicImpl) Format() string {
	return s.formatID
}

func (s *SchematicImpl) DataVersion() int {
	return s.dataVersion
}

func (s *SchematicImpl) SetDataVersion(version int) {
	s.dataVersion = version
}

func (s *SchematicImpl) Version() string {
	switch {
	case s.dataVersion >= 4665:
		return "1.21.11"
	case s.dataVersion >= 4556:
		return "1.21.10"
	case s.dataVersion >= 4554:
		return "1.21.9"
	case s.dataVersion >= 4440:
		return "1.21.8"
	case s.dataVersion >= 4438:
		return "1.21.7"
	case s.dataVersion >= 4435:
		return "1.21.6"
	case s.dataVersion >= 4325:
		return "1.21.5"
	case s.dataVersion >= 4189:
		return "1.21.4"
	case s.dataVersion >= 4082:
		return "1.21.3"
	case s.dataVersion >= 4080:
		return "1.21.2"
	case s.dataVersion >= 3955:
		return "1.21.1"
	case s.dataVersion >= 3953:
		return "1.21"
	case s.dataVersion >= 3839:
		return "1.20.6"
	case s.dataVersion >= 3837:
		return "1.20.5"
	case s.dataVersion >= 3700:
		return "1.20.4"
	case s.dataVersion >= 3578:
		return "1.20.2"
	case s.dataVersion >= 3465:
		return "1.20.1"
	case s.dataVersion >= 3463:
		return "1.20"
	case s.dataVersion >= 3337:
		return "1.19.4"
	case s.dataVersion >= 3218:
		return "1.19.3"
	case s.dataVersion >= 3120:
		return "1.19.2"
	case s.dataVersion >= 3117:
		return "1.19.1"
	case s.dataVersion >= 3105:
		return "1.19"
	case s.dataVersion >= 2975:
		return "1.18.2"
	case s.dataVersion >= 2860:
		return "1.18"
	case s.dataVersion >= 2730:
		return "1.17.1"
	case s.dataVersion >= 2724:
		return "1.17"
	case s.dataVersion >= 2586:
		return "1.16.5"
	case s.dataVersion >= 2566:
		return "1.16"
	case s.dataVersion >= 2230:
		return "1.15.2"
	case s.dataVersion >= 2225:
		return "1.15"
	case s.dataVersion >= 1976:
		return "1.14.4"
	case s.dataVersion >= 1952:
		return "1.14"
	case s.dataVersion >= 1631:
		return "1.13.2"
	case s.dataVersion >= 1628:
		return "1.13.1"
	case s.dataVersion >= 1519:
		return "1.13"
	case s.dataVersion >= 1343:
		return "1.12.2"
	case s.dataVersion >= 1241:
		return "1.12.1"
	case s.dataVersion >= 1139:
		return "1.12"
	case s.dataVersion >= 922:
		return "1.11.2"
	case s.dataVersion >= 921:
		return "1.11.1"
	case s.dataVersion >= 819:
		return "1.11"
	case s.dataVersion >= 512:
		return "1.10.2"
	case s.dataVersion >= 511:
		return "1.10.1"
	case s.dataVersion >= 510:
		return "1.10"
	case s.dataVersion >= 184:
		return "1.9.4"
	case s.dataVersion >= 183:
		return "1.9.3"
	case s.dataVersion >= 176:
		return "1.9.2"
	case s.dataVersion >= 175:
		return "1.9.1"
	case s.dataVersion >= 169:
		return "1.9"
	default:
		return ""
	}
}
