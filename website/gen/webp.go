package main

import (
	"encoding/binary"
	"fmt"
)

// webpSize reads the canvas size from a WebP header so screenshots get
// width/height attributes and the layout does not shift while they load.
func webpSize(b []byte) (int, int, error) {
	if len(b) < 30 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WEBP" {
		return 0, 0, fmt.Errorf("not a WebP file")
	}
	switch string(b[12:16]) {
	case "VP8X":
		w := int(b[24]) | int(b[25])<<8 | int(b[26])<<16
		h := int(b[27]) | int(b[28])<<8 | int(b[29])<<16
		return w + 1, h + 1, nil
	case "VP8 ":
		if b[23] != 0x9d || b[24] != 0x01 || b[25] != 0x2a {
			return 0, 0, fmt.Errorf("bad VP8 start code")
		}
		w := binary.LittleEndian.Uint16(b[26:28]) & 0x3fff
		h := binary.LittleEndian.Uint16(b[28:30]) & 0x3fff
		return int(w), int(h), nil
	case "VP8L":
		if b[20] != 0x2f {
			return 0, 0, fmt.Errorf("bad VP8L signature")
		}
		bits := binary.LittleEndian.Uint32(b[21:25])
		return int(bits&0x3fff) + 1, int(bits>>14&0x3fff) + 1, nil
	}
	return 0, 0, fmt.Errorf("unknown WebP chunk %q", b[12:16])
}
