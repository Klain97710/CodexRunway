//go:build linux

package usage

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

const statxMask = unix.STATX_TYPE | unix.STATX_INO | unix.STATX_SIZE | unix.STATX_MTIME | unix.STATX_CTIME | unix.STATX_BTIME

func lstatMeta(path string) (fileMeta, error) {
	var sx unix.Statx_t
	err := unix.Statx(unix.AT_FDCWD, path, unix.AT_SYMLINK_NOFOLLOW, statxMask, &sx)
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EPERM) {
		fi, lerr := os.Lstat(path)
		if lerr != nil {
			return fileMeta{}, lerr
		}
		return metaFromInfo(fi)
	}
	if err != nil {
		return fileMeta{}, &os.PathError{Op: "statx", Path: path, Err: err}
	}
	return metaFromStatx(&sx)
}

func fstatMeta(f *os.File) (fileMeta, error) {
	var sx unix.Statx_t
	err := unix.Statx(int(f.Fd()), "", unix.AT_EMPTY_PATH, statxMask, &sx)
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EPERM) {
		fi, serr := f.Stat()
		if serr != nil {
			return fileMeta{}, serr
		}
		return metaFromInfo(fi)
	}
	if err != nil {
		return fileMeta{}, &os.PathError{Op: "statx", Path: f.Name(), Err: err}
	}
	return metaFromStatx(&sx)
}

func metaFromStatx(sx *unix.Statx_t) (fileMeta, error) {
	var m fileMeta
	var err error
	m.Regular = sx.Mode&unix.S_IFMT == unix.S_IFREG
	m.Device = int64(unix.Mkdev(sx.Dev_major, sx.Dev_minor))
	m.Inode = int64(sx.Ino)
	m.Size = sx.Size
	if sx.Mask&unix.STATX_BTIME != 0 {
		if m.BirthNS, err = nanosecondsOf(sx.Btime.Sec, int64(sx.Btime.Nsec)); err != nil {
			return fileMeta{}, err
		}
	}
	if m.MtimeNS, err = nanosecondsOf(sx.Mtime.Sec, int64(sx.Mtime.Nsec)); err != nil {
		return fileMeta{}, err
	}
	if m.CtimeNS, err = nanosecondsOf(sx.Ctime.Sec, int64(sx.Ctime.Nsec)); err != nil {
		return fileMeta{}, err
	}
	return m, nil
}

// metaFromInfo is the fallback for kernels without statx; the birth time is
// then unknown (0).
func metaFromInfo(fi os.FileInfo) (fileMeta, error) {
	st, ok := fi.Sys().(*statT)
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
	if m.MtimeNS, err = nanosecondsOf(int64(st.Mtim.Sec), int64(st.Mtim.Nsec)); err != nil {
		return fileMeta{}, err
	}
	if m.CtimeNS, err = nanosecondsOf(int64(st.Ctim.Sec), int64(st.Ctim.Nsec)); err != nil {
		return fileMeta{}, err
	}
	return m, nil
}
