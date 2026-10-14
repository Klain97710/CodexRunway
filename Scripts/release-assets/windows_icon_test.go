package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"strings"
	"testing"
)

func iconResources(t *testing.T) peResources {
	t.Helper()
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 256, 256))); err != nil {
		t.Fatal(err)
	}
	raw := make([]byte, 192+imageData.Len())
	le := binary.LittleEndian
	directory := func(at int, entries [][2]uint32) {
		le.PutUint16(raw[at+14:], uint16(len(entries)))
		for i, entry := range entries {
			le.PutUint32(raw[at+16+8*i:], entry[0])
			le.PutUint32(raw[at+20+8*i:], entry[1])
		}
	}
	directory(0, [][2]uint32{{3, 32 | 0x80000000}, {14, 80 | 0x80000000}})
	directory(32, [][2]uint32{{7, 56 | 0x80000000}})
	directory(56, [][2]uint32{{0x409, 128}})
	directory(80, [][2]uint32{{1, 104 | 0x80000000}})
	directory(104, [][2]uint32{{0x409, 144}})
	le.PutUint32(raw[128:], 0x1000+192)
	le.PutUint32(raw[132:], uint32(imageData.Len()))
	le.PutUint32(raw[144:], 0x1000+160)
	le.PutUint32(raw[148:], 20)
	le.PutUint16(raw[162:], 1)
	le.PutUint16(raw[164:], 1)
	le.PutUint16(raw[170:], 1)
	le.PutUint16(raw[172:], 32)
	le.PutUint32(raw[174:], uint32(imageData.Len()))
	le.PutUint16(raw[178:], 7)
	copy(raw[192:], imageData.Bytes())
	return peResources{data: raw, rva: 0x1000}
}
func TestWindowsIconUsesActualResourceGroup(t *testing.T) {
	resources := iconResources(t)
	if err := resources.verifyIcon(); err != nil {
		t.Fatal(err)
	}
	binary.LittleEndian.PutUint32(resources.data[24:], 15)
	if err := resources.verifyIcon(); err == nil {
		t.Fatal("embedded PNG accepted without RT_GROUP_ICON")
	}
}
func TestWindowsEmbeddedPNGIsNotAnIconResource(t *testing.T) {
	resources := iconResources(t)
	raw := make([]byte, 128)
	copy(raw, "MZ")
	binary.LittleEndian.PutUint32(raw[60:], 64)
	copy(raw[64:], "PE\x00\x00")
	binary.LittleEndian.PutUint16(raw[68:], 0x8664)
	binary.LittleEndian.PutUint16(raw[70:], 1)
	copy(raw[88:], ".text")
	binary.LittleEndian.PutUint32(raw[104:], uint32(len(resources.data)))
	binary.LittleEndian.PutUint32(raw[108:], 128)
	raw = append(raw, resources.data...)
	err := verifyWindowsIcon(raw)
	if err == nil || !strings.Contains(err.Error(), "resource section missing") {
		t.Fatalf("missing icon resource not detected: %v", err)
	}
}
