//go:build darwin

package usage

import (
	"os"
	"syscall"
)

func lstatMeta(path string) (fileMeta, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return fileMeta{}, err
	}
	return metaFromInfo(fi)
}

func fstatMeta(f *os.File) (fileMeta, error) {
	fi, err := f.Stat()
	if err != nil {
		return fileMeta{}, err
	}
	return metaFromInfo(fi)
}

func metaFromInfo(fi os.FileInfo) (fileMeta, error) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return fileMeta{}, ErrInvalidSourceFile
	}
	var m fileMeta
	var err error
	m.Regular = fi.Mode().IsRegular()
	m.Device = int64(st.Dev)
	m.Inode = int64(st.Ino)
	if st.Size < 0 {
		return fileMeta{}, ErrInvalidSourceFile
	}
	m.Size = uint64(st.Size)
	if m.BirthNS, err = nanosecondsOf(int64(st.Birthtimespec.Sec), int64(st.Birthtimespec.Nsec)); err != nil {
		return fileMeta{}, err
	}
	if m.MtimeNS, err = nanosecondsOf(int64(st.Mtimespec.Sec), int64(st.Mtimespec.Nsec)); err != nil {
		return fileMeta{}, err
	}
	if m.CtimeNS, err = nanosecondsOf(int64(st.Ctimespec.Sec), int64(st.Ctimespec.Nsec)); err != nil {
		return fileMeta{}, err
	}
	return m, nil
}
