package mc

import (
	"fmt"
	"io"
)

const maxVarIntBytes = 5

func writeVarInt(w io.Writer, value int32) error {
	uv := uint32(value)
	for {
		b := byte(uv & 0x7F)
		uv >>= 7
		if uv != 0 {
			b |= 0x80
		}
		if _, err := w.Write([]byte{b}); err != nil {
			return err
		}
		if uv == 0 {
			return nil
		}
	}
}

func readVarInt(r io.Reader) (int32, error) {
	var result int32
	var numRead uint
	buf := make([]byte, 1)
	for {
		if _, err := io.ReadFull(r, buf); err != nil {
			return 0, err
		}
		result |= int32(buf[0]&0x7F) << (7 * numRead)
		numRead++
		if numRead > maxVarIntBytes {
			return 0, fmt.Errorf("varint is too long")
		}
		if buf[0]&0x80 == 0 {
			return result, nil
		}
	}
}

func writeString(w io.Writer, s string) error {
	if err := writeVarInt(w, int32(len(s))); err != nil {
		return err
	}
	_, err := w.Write([]byte(s))
	return err
}
