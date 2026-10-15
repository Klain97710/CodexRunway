package claude

import (
	"sync"
	"time"
)

// SessionCache keeps the parse of each session log between scans. A file is
// read again only when its size or modification time changed, so a refresh
// re-reads the active sessions and not the whole projects tree. Entries of
// files no scan listed are dropped. It is safe for concurrent use.
type SessionCache struct {
	mu    sync.Mutex
	files map[string]cachedSessionFile
}

type cachedSessionFile struct {
	size  int64
	mtime time.Time
	part  *scanSink
}

func (c *SessionCache) get(file sessionFile) (*scanSink, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	hit, ok := c.files[file.path]
	if !ok || hit.size != file.size || !hit.mtime.Equal(file.mtime) {
		return nil, false
	}
	return hit.part, true
}

func (c *SessionCache) put(file sessionFile, part *scanSink) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.files == nil {
		c.files = map[string]cachedSessionFile{}
	}
	c.files[file.path] = cachedSessionFile{size: file.size, mtime: file.mtime, part: part}
}

// retain drops entries for files outside listed.
func (c *SessionCache) retain(listed map[string]bool) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for path := range c.files {
		if !listed[path] {
			delete(c.files, path)
		}
	}
}
