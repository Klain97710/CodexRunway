package usage

import "sync"

// Diagnostics counts the work the repository did, for tests and a debug view.
type Diagnostics struct {
	BytesRead               int
	ValidationBytesRead     int
	CandidateLines          int
	DecodedLines            int
	MalformedCandidateLines int
	OversizedLines          int
	MaxBufferedBytes        int
	RebuiltFiles            int
	IndexPasses             int
	CacheHits               int
	AppendedFiles           int
	IncompleteTailFiles     int
	AdoptedFiles            int
	DuplicateFiles          int
	RemovedFiles            int
	DatabaseRebuilds        int
	MaxConcurrentScans      int
	SharedFlightHits        int
	CancelledFlights        int
}

// diagState is a Diagnostics guarded for concurrent update and snapshot.
type diagState struct {
	mu sync.Mutex
	d  Diagnostics
}

func (s *diagState) update(fn func(*Diagnostics)) {
	s.mu.Lock()
	fn(&s.d)
	s.mu.Unlock()
}

func (s *diagState) snapshot() Diagnostics {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.d
}

// scanMetrics is what one pass over a file contributes to Diagnostics.
type scanMetrics struct {
	stream    streamResult
	hashBytes int
}

func (m scanMetrics) record(d *Diagnostics) {
	d.BytesRead += m.stream.BytesRead
	d.ValidationBytesRead += m.hashBytes
	d.CandidateLines += m.stream.CandidateLines
	d.DecodedLines += m.stream.DecodedLines
	d.MalformedCandidateLines += m.stream.MalformedLines
	d.OversizedLines += m.stream.OversizedLines
	d.MaxBufferedBytes = max(d.MaxBufferedBytes, m.stream.MaxBufferedBytes)
	if m.stream.HasTrailingLine {
		d.IncompleteTailFiles++
	}
}

// anomalies are line counts that are provisional because they sit in an
// incomplete trailing line, which a later append may complete.
type anomalies struct {
	Malformed int64
	Oversized int64
}

func (a anomalies) add(o anomalies) (anomalies, error) {
	m, err := checkedAdd(a.Malformed, o.Malformed, "malformed line count")
	if err != nil {
		return a, err
	}
	v, err := checkedAdd(a.Oversized, o.Oversized, "oversized line count")
	if err != nil {
		return a, err
	}
	return anomalies{m, v}, nil
}

func (m scanMetrics) provisional() anomalies {
	return anomalies{Malformed: int64(m.stream.IncompleteMalformed), Oversized: int64(m.stream.IncompleteOversized)}
}
