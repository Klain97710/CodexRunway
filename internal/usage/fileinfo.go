package usage

import (
	"errors"
	"math"
)

// fileMeta is the identity and change-detection metadata of a source file. The
// numbers follow the stat(2) values the index stores: device and inode form
// the identity together with the birth time; size, mtime and ctime detect
// change.
type fileMeta struct {
	Device  int64
	Inode   int64
	BirthNS int64
	MtimeNS int64
	CtimeNS int64
	Size    uint64
	Regular bool
}

var (
	errTimestampOverflow = errors.New("usage: file timestamp overflow")
	// ErrInvalidSourceFile reports a source that is not a regular readable file.
	ErrInvalidSourceFile = errors.New("usage: invalid source file")
	// ErrSourceIdentityChanged reports that the file at a path is no longer the
	// file that was inventoried (replaced or rotated).
	ErrSourceIdentityChanged = errors.New("usage: source identity changed")
	// ErrSourceMetadataChanged reports that a file changed while it was read.
	ErrSourceMetadataChanged = errors.New("usage: source metadata changed")
	// ErrOffsetBeyondSnapshot reports a resume offset past the end of the file.
	ErrOffsetBeyondSnapshot = errors.New("usage: offset beyond file snapshot")
	// ErrUnexpectedEOF reports that a file shrank while it was read.
	ErrUnexpectedEOF = errors.New("usage: unexpected end of file")
)

// nanosecondsOf converts Unix seconds and nanoseconds to nanoseconds with the
// overflow check the index relies on.
func nanosecondsOf(sec, nsec int64) (int64, error) {
	const billion = 1_000_000_000
	if sec > math.MaxInt64/billion || sec < math.MinInt64/billion {
		return 0, errTimestampOverflow
	}
	s := sec * billion
	r := s + nsec
	if (r > s) != (nsec > 0) {
		return 0, errTimestampOverflow
	}
	return r, nil
}

// sameIdentity reports whether b describes the file a was taken from.
func (a fileMeta) sameIdentity(b fileMeta) bool {
	return a.Device == b.Device && a.Inode == b.Inode && a.BirthNS == b.BirthNS
}

// sameSnapshot reports whether nothing observable changed between two looks.
func (a fileMeta) sameSnapshot(b fileMeta) bool {
	return a.sameIdentity(b) && a.Size == b.Size && a.MtimeNS == b.MtimeNS && a.CtimeNS == b.CtimeNS
}
