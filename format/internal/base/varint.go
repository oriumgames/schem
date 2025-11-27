package base

import (
	"fmt"
	"io"
)

// DecodeVarInt reads a single VarInt from the byte slice.
// Returns the value and the number of bytes read.
func DecodeVarInt(data []byte) (int, int, error) {
	var value, length int
	for {
		if length >= len(data) {
			return 0, 0, fmt.Errorf("varint extends beyond data")
		}
		b := int(data[length])
		value |= (b & 0x7F) << (length * 7)
		length++
		if length > 5 {
			return 0, 0, fmt.Errorf("varint too long")
		}
		if (b & 0x80) == 0 {
			break
		}
	}
	return value, length, nil
}

// DecodeVarIntArray decodes multiple VarInts from a byte slice.
func DecodeVarIntArray(data []byte, count int) ([]int, error) {
	values := make([]int, count)
	offset := 0
	for i := range count {
		val, length, err := DecodeVarInt(data[offset:])
		if err != nil {
			return nil, fmt.Errorf("decode varint %d: %w", i, err)
		}
		values[i] = val
		offset += length
	}
	return values, nil
}

// EncodeVarInt encodes a single integer as a VarInt.
func EncodeVarInt(value int) []byte {
	var buf []byte
	for {
		b := byte(value & 0x7F)
		value >>= 7
		if value != 0 {
			b |= 0x80
		}
		buf = append(buf, b)
		if value == 0 {
			break
		}
	}
	return buf
}

// EncodeVarIntArray encodes multiple integers as VarInts.
func EncodeVarIntArray(values []int) []byte {
	var buf []byte
	for _, v := range values {
		buf = append(buf, EncodeVarInt(v)...)
	}
	return buf
}

// ReadVarInt reads a VarInt from an io.Reader.
func ReadVarInt(r io.Reader) (int, error) {
	var value, shift int
	var b [1]byte
	for {
		if _, err := r.Read(b[:]); err != nil {
			return 0, err
		}
		value |= int(b[0]&0x7F) << shift
		if (b[0] & 0x80) == 0 {
			break
		}
		shift += 7
		if shift > 35 {
			return 0, fmt.Errorf("varint too long")
		}
	}
	return value, nil
}

// WriteVarInt writes a VarInt to an io.Writer.
func WriteVarInt(w io.Writer, value int) error {
	buf := EncodeVarInt(value)
	_, err := w.Write(buf)
	return err
}
