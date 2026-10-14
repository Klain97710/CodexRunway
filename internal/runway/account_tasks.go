package runway

import "context"

type accountTask struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// Register before spawning so provider/account switches can cancel
// standalone work; epochs isolate late results without delaying a switch.
func (s *Service) runAccountScoped(fn func(context.Context)) <-chan struct{} {
	s.mu.Lock()
	ctx, cancel := context.WithCancel(s.root)
	s.standaloneID++
	id := s.standaloneID
	done := make(chan struct{})
	s.standalone[id] = accountTask{cancel: cancel, done: done}
	s.mu.Unlock()
	finish := func() {
		cancel()
		s.mu.Lock()
		delete(s.standalone, id)
		s.mu.Unlock()
		close(done)
	}
	if !s.spawn(func() {
		defer finish()
		if ctx.Err() == nil {
			fn(ctx)
		}
	}) {
		finish()
	}
	return done
}

// A late standalone completion cannot clear the new account's busy flags.
func (s *Service) leaveAccount(epoch uint64, sections ...Section) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if epoch != s.epoch {
		return
	}
	for _, section := range sections {
		delete(s.busy, section)
	}
	s.publishLocked()
}

func (s *Service) clearAccountBusyLocked() {
	for _, section := range []Section{SectionQuota, SectionResetCredits, SectionQuotaEstimate, SectionAPICost} {
		delete(s.busy, section)
	}
}
