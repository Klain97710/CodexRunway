package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

func verifyArchive(file, target string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	system, arch, _ := strings.Cut(target, "-")
	executable := "quosparr"
	sqlite := "libmygo-sqlite3.so"
	if system == "windows" {
		executable = "Quosparr.exe"
		sqlite = "mygo-sqlite3.dll"
	}
	if system == "darwin" {
		executable = "Quosparr.app/Contents/MacOS/Quosparr"
		sqlite = "Quosparr.app/Contents/Resources/libmygo-sqlite3.dylib"
	}
	seen := map[string]bool{}
	links := map[string]string{}
	required := map[string]bool{executable: false, sqlite: false}
	if system == "windows" {
		delete(required, sqlite)
	}
	if system == "darwin" {
		required["Quosparr.app/Contents/Frameworks/libQuosparrWidgetBridge.dylib"] = false
		required["Quosparr.app/Contents/PlugIns/QuosparrWidget.appex/Contents/MacOS/QuosparrWidget"] = false
	}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := entryName(hdr.Name)
		if err := safeEntry(hdr); err != nil {
			return err
		}
		if seen[name] {
			return fmt.Errorf("duplicate archive entry %s", name)
		}
		seen[name] = true
		if system == "windows" && path.Base(name) == sqlite {
			return errors.New("Windows archive must embed SQLite instead of shipping a separate DLL")
		}
		if hdr.Typeflag == tar.TypeSymlink {
			links[name] = hdr.Linkname
		}
		if _, ok := required[name]; !ok {
			continue
		}
		if hdr.Typeflag != tar.TypeReg {
			return fmt.Errorf("%s must be a regular file", name)
		}
		if system != "windows" && hdr.Mode&0111 == 0 {
			return fmt.Errorf("%s has no executable permission", name)
		}
		if hdr.Size > 256<<20 {
			return errors.New("archive binary exceeds validation limit")
		}
		data, err := io.ReadAll(io.LimitReader(tr, 256<<20))
		if err != nil {
			return err
		}
		if system == "windows" && name == executable {
			if err := verifyWindowsIcon(data); err != nil {
				return err
			}
			if err := verifyEmbeddedSQLite(data, arch); err != nil {
				return err
			}
		}
		if err := verifyArchitecture(data, system, arch); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		required[name] = true
	}
	if err := verifyLinks(seen, links); err != nil {
		return err
	}
	for name, found := range required {
		if !found {
			return fmt.Errorf("missing archive payload %s", name)
		}
	}
	return nil
}
func entryName(name string) string {
	for strings.HasPrefix(name, "./") {
		name = strings.TrimPrefix(name, "./")
	}
	return strings.TrimSuffix(name, "/")
}
func safeEntry(hdr *tar.Header) error {
	name := entryName(hdr.Name)
	if name == "" || name == "." {
		if hdr.Typeflag == tar.TypeDir {
			return nil
		}
		return errors.New("invalid root tar entry")
	}
	if path.IsAbs(name) || name == ".." || strings.HasPrefix(name, "../") || path.Clean(name) != name || strings.Contains(name, "\\") {
		return errors.New("unsafe tar entry")
	}
	if hdr.Typeflag == tar.TypeSymlink {
		resolved := path.Clean(path.Join(path.Dir(name), hdr.Linkname))
		if path.IsAbs(hdr.Linkname) || resolved == ".." || strings.HasPrefix(resolved, "../") || strings.Contains(hdr.Linkname, "\\") {
			return errors.New("unsafe tar symlink")
		}
	}
	switch hdr.Typeflag {
	case tar.TypeReg, tar.TypeDir, tar.TypeSymlink:
	default:
		return errors.New("unsupported tar entry")
	}
	return nil
}
func verifyLinks(seen map[string]bool, links map[string]string) error {
	for name := range seen {
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if _, ok := links[parent]; ok {
				return errors.New("archive entry traverses a symlink")
			}
		}
	}
	for name, target := range links {
		resolved := path.Clean(path.Join(path.Dir(name), target))
		for count := 0; ; count++ {
			if count > len(links) {
				return errors.New("cyclic archive symlink")
			}
			replaced := false
			for prefix := resolved; prefix != "."; prefix = path.Dir(prefix) {
				if next, ok := links[prefix]; ok {
					tail := strings.TrimPrefix(resolved, prefix)
					resolved = path.Clean(path.Join(path.Dir(prefix), next) + tail)
					replaced = true
					break
				}
			}
			if resolved == ".." || strings.HasPrefix(resolved, "../") || path.IsAbs(resolved) {
				return errors.New("symlink chain escapes archive")
			}
			if !replaced {
				break
			}
		}
		if !seen[resolved] {
			return errors.New("archive symlink target missing")
		}
	}
	return nil
}
func verifyArchitecture(data []byte, system, arch string) error {
	reader := bytes.NewReader(data)
	switch system {
	case "darwin":
		expected := macho.CpuArm64
		if arch == "amd64" {
			expected = macho.CpuAmd64
		}
		f, err := macho.NewFile(reader)
		if err == nil {
			if f.Cpu != expected {
				return errors.New("Mach-O architecture mismatch")
			}
			return nil
		}
		fat, fatErr := macho.NewFatFile(reader)
		if fatErr != nil {
			return err
		}
		for _, slice := range fat.Arches {
			if slice.Cpu == expected {
				return nil
			}
		}
		return errors.New("universal Mach-O is missing expected architecture")
	case "windows":
		f, err := pe.NewFile(reader)
		if err != nil {
			return err
		}
		expected := uint16(pe.IMAGE_FILE_MACHINE_ARM64)
		if arch == "amd64" {
			expected = pe.IMAGE_FILE_MACHINE_AMD64
		}
		if f.Machine != expected {
			return errors.New("PE architecture mismatch")
		}
	case "linux":
		f, err := elf.NewFile(reader)
		if err != nil {
			return err
		}
		expected := elf.EM_AARCH64
		if arch == "amd64" {
			expected = elf.EM_X86_64
		}
		if f.Machine != expected {
			return errors.New("ELF architecture mismatch")
		}
	}
	return nil
}
