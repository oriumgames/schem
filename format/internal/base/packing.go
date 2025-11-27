package base

import "math"

// CalculateBitsPerEntry calculates the minimum bits per entry needed for a palette.
func CalculateBitsPerEntry(paletteSize int) int {
	if paletteSize <= 1 {
		return 1
	}
	return max(int(math.Ceil(math.Log2(float64(paletteSize)))), 2)
}

// PackLongArray packs values into a long array using standard Minecraft packing.
// Standard packing: values don't cross long boundaries.
func PackLongArray(values []int, bitsPerEntry int) []int64 {
	if bitsPerEntry == 0 {
		return nil
	}
	valuesPerLong := 64 / bitsPerEntry
	longCount := (len(values) + valuesPerLong - 1) / valuesPerLong
	longs := make([]int64, longCount)

	for i, val := range values {
		longIdx := i / valuesPerLong
		bitIdx := (i % valuesPerLong) * bitsPerEntry
		longs[longIdx] |= int64(val) << bitIdx
	}
	return longs
}

// UnpackLongArray unpacks values from a long array using standard Minecraft packing.
func UnpackLongArray(longs []int64, bitsPerEntry, count int) []int {
	if bitsPerEntry == 0 || len(longs) == 0 {
		return make([]int, count)
	}
	valuesPerLong := 64 / bitsPerEntry
	mask := (1 << bitsPerEntry) - 1
	values := make([]int, count)

	for i := range count {
		longIdx := i / valuesPerLong
		bitIdx := (i % valuesPerLong) * bitsPerEntry
		if longIdx < len(longs) {
			values[i] = int((longs[longIdx] >> bitIdx) & int64(mask))
		}
	}
	return values
}

// PackLongArrayTight packs values using Litematica's tight packing.
// Tight packing: values can cross long boundaries for maximum space efficiency.
func PackLongArrayTight(values []int, bitsPerEntry int) []int64 {
	if bitsPerEntry == 0 {
		return nil
	}
	totalBits := len(values) * bitsPerEntry
	longCount := (totalBits + 63) / 64
	longs := make([]int64, longCount)

	bitPos := 0
	for _, val := range values {
		longIdx := bitPos / 64
		bitOffset := bitPos % 64

		bitsInFirstLong := 64 - bitOffset
		if bitsInFirstLong >= bitsPerEntry {
			// Entire value fits in current long
			longs[longIdx] |= int64(val) << bitOffset
		} else {
			// Value spans two longs
			mask1 := (1 << bitsInFirstLong) - 1
			longs[longIdx] |= int64(val&mask1) << bitOffset
			if longIdx+1 < len(longs) {
				bitsInSecondLong := bitsPerEntry - bitsInFirstLong
				longs[longIdx+1] |= int64(val>>bitsInFirstLong) & ((1 << bitsInSecondLong) - 1)
			}
		}
		bitPos += bitsPerEntry
	}
	return longs
}

// UnpackLongArrayTight unpacks values using Litematica's tight packing.
func UnpackLongArrayTight(longs []int64, bitsPerEntry, count int) []int {
	if bitsPerEntry == 0 || len(longs) == 0 {
		return make([]int, count)
	}
	values := make([]int, count)
	mask := (1 << bitsPerEntry) - 1

	bitPos := 0
	for i := range count {
		longIdx := bitPos / 64
		bitOffset := bitPos % 64

		if longIdx >= len(longs) {
			break
		}

		bitsInFirstLong := 64 - bitOffset
		if bitsInFirstLong >= bitsPerEntry {
			// Entire value in current long
			values[i] = int((longs[longIdx] >> bitOffset) & int64(mask))
		} else {
			// Value spans two longs
			mask1 := (1 << bitsInFirstLong) - 1
			val := int((longs[longIdx] >> bitOffset) & int64(mask1))
			if longIdx+1 < len(longs) {
				bitsInSecondLong := bitsPerEntry - bitsInFirstLong
				mask2 := (1 << bitsInSecondLong) - 1
				val |= int(longs[longIdx+1]&int64(mask2)) << bitsInFirstLong
			}
			values[i] = val
		}
		bitPos += bitsPerEntry
	}
	return values
}
