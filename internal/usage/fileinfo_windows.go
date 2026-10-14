//go:build windows

package usage

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// On Windows the identity is the volume serial plus the 64-bit file index, and
// the times are the creation, last-write and change times of the file.

func lstatMeta(path string) (fileMeta, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return fileMeta{}, err
	}
	if !fi.Mode().IsRegular() {
		return fileMeta{Regular: false}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return fileMeta{}, err
	}
	defer f.Close()
	return fstatMeta(f)
}

func fstatMeta(f *os.File) (fileMeta, error) {
	h := windows.Handle(f.Fd())
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		return fileMeta{}, err
	}
	var basic struct {
		CreationTime   int64
		LastAccessTime int64
		LastWriteTime  int64
		ChangeTime     int64
		FileAttributes uint32
		_              uint32
	}
	if err := windows.GetFileInformationByHandleEx(h, windows.FileBasicInfo, (*byte)(unsafe.Pointer(&basic)), uint32(unsafe.Sizeof(basic))); err != nil {
		return fileMeta{}, err
	}
	m := fileMeta{
		Device:  int64(info.VolumeSerialNumber),
		Inode:   int64(uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow)),
		Size:    uint64(info.FileSizeHigh)<<32 | uint64(info.FileSizeLow),
		Regular: info.FileAttributes&(windows.FILE_ATTRIBUTE_DIRECTORY|windows.FILE_ATTRIBUTE_REPARSE_POINT) == 0,
		BirthNS: fileTimeNS(basic.CreationTime),
		MtimeNS: fileTimeNS(basic.LastWriteTime),
		CtimeNS: fileTimeNS(basic.ChangeTime),
	}
	return m, nil
}

// fileTimeNS converts a FILETIME (100 ns ticks since 1601) to Unix nanoseconds.
func fileTimeNS(ft int64) int64 {
	const epochDiff = 116444736000000000
	return (ft - epochDiff) * 100
}
