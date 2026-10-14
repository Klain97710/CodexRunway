package main

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"errors"
	"image/png"
)

type peResources struct {
	data      []byte
	rva, root uint32
}

func verifyWindowsIcon(data []byte) error {
	file, err := pe.NewFile(bytes.NewReader(data))
	if err != nil {
		return err
	}
	section := file.Section(".rsrc")
	if section == nil {
		return errors.New("Windows icon resource section missing")
	}
	raw, err := section.Data()
	if err != nil {
		return err
	}
	var root uint32
	switch header := file.OptionalHeader.(type) {
	case *pe.OptionalHeader64:
		root = header.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_RESOURCE].VirtualAddress
	case *pe.OptionalHeader32:
		root = header.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_RESOURCE].VirtualAddress
	default:
		return errors.New("Windows resource directory missing")
	}
	if root < section.VirtualAddress {
		return errors.New("invalid Windows resource RVA")
	}
	resources := peResources{data: raw, rva: section.VirtualAddress, root: root - section.VirtualAddress}
	return resources.verifyIcon()
}
func (r peResources) rangeAt(offset, size uint32) ([]byte, error) {
	if uint64(offset)+uint64(size) > uint64(len(r.data)) {
		return nil, errors.New("Windows resource exceeds section")
	}
	return r.data[offset : offset+size], nil
}
func (r peResources) directory(offset, id uint32) (uint32, error) {
	header, err := r.rangeAt(offset, 16)
	if err != nil {
		return 0, err
	}
	count := uint32(binary.LittleEndian.Uint16(header[12:])) + uint32(binary.LittleEndian.Uint16(header[14:]))
	entries, err := r.rangeAt(offset+16, count*8)
	if err != nil {
		return 0, err
	}
	for i := uint32(0); i < count; i++ {
		if binary.LittleEndian.Uint32(entries[i*8:]) == id {
			return binary.LittleEndian.Uint32(entries[i*8+4:]), nil
		}
	}
	return 0, errors.New("required Windows icon resource missing")
}
func (r peResources) blob(kind, id uint32) ([]byte, error) {
	offset := r.root
	for _, value := range []uint32{kind, id, 0x409} {
		next, err := r.directory(offset, value)
		if err != nil {
			return nil, err
		}
		if value != 0x409 && next&0x80000000 == 0 {
			return nil, errors.New("Windows icon directory malformed")
		}
		if value == 0x409 && next&0x80000000 != 0 {
			return nil, errors.New("Windows icon data malformed")
		}
		offset = r.root + (next & 0x7fffffff)
	}
	entry, err := r.rangeAt(offset, 16)
	if err != nil {
		return nil, err
	}
	rva, size := binary.LittleEndian.Uint32(entry), binary.LittleEndian.Uint32(entry[4:])
	if rva < r.rva {
		return nil, errors.New("invalid Windows icon data RVA")
	}
	return r.rangeAt(rva-r.rva, size)
}
func (r peResources) verifyIcon() error {
	group, err := r.blob(14, 1)
	if err != nil {
		return err
	}
	if len(group) < 6 || binary.LittleEndian.Uint16(group) != 0 || binary.LittleEndian.Uint16(group[2:]) != 1 {
		return errors.New("invalid RT_GROUP_ICON header")
	}
	count := int(binary.LittleEndian.Uint16(group[4:]))
	if len(group) != 6+14*count {
		return errors.New("invalid RT_GROUP_ICON length")
	}
	for i := 0; i < count; i++ {
		entry := group[6+i*14 : 6+(i+1)*14]
		if entry[0] != 0 || entry[1] != 0 {
			continue
		}
		image, err := r.blob(3, uint32(binary.LittleEndian.Uint16(entry[12:])))
		if err != nil {
			return err
		}
		if len(image) != int(binary.LittleEndian.Uint32(entry[8:])) {
			return errors.New("RT_ICON length mismatch")
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(image))
		if err != nil {
			return err
		}
		if cfg.Width != 256 || cfg.Height != 256 {
			return errors.New("RT_ICON is not 256px")
		}
		return nil
	}
	return errors.New("RT_GROUP_ICON has no 256px icon")
}
