package voicematch

import (
	"context"
	"errors"
	"log/slog"
	"slices"

	"github.com/ygelfand/libcountertop/pkg/runtime/safe"
)

var ErrEnrolling = errors.New("voicematch: an enrolment is already running")

type Outcome int

const (
	Enrolling Outcome = iota + 1
	Ready
	Unconfirmed
	Failed
)

type Progress struct {
	ID      string
	Name    string
	Step    Step
	Outcome Outcome
	Score   float64
	Err     error
}

func (s *Service) Begin(id, name, person string) error {
	s.mu.Lock()
	if s.stop != nil {
		s.mu.Unlock()
		return ErrEnrolling
	}
	ctx, stop := context.WithCancel(context.Background())
	s.stop = stop
	s.progress = Progress{ID: id, Name: name, Outcome: Enrolling}
	s.mu.Unlock()
	s.Changed.Emit(s.Progress())

	safe.Go("voice enrolment", func() {
		defer stop()
		p, m, err := s.Enroll(ctx, id, name, person, func(st Step) {
			s.update(func(pr *Progress) { pr.Step = st })
		})
		if err == nil {
			err = s.Save(p)
		}
		s.mu.Lock()
		s.stop = nil
		s.mu.Unlock()
		if errors.Is(err, context.Canceled) {
			s.update(func(pr *Progress) { *pr = Progress{} })
			return
		}
		s.update(func(pr *Progress) {
			pr.Score, pr.Err = m.Score, err
			switch {
			case err == nil:
				pr.Outcome = Ready
			case errors.Is(err, ErrNotConfirmed):
				pr.Outcome = Unconfirmed
			default:
				pr.Outcome = Failed
			}
		})
		slog.Info("voice enrolment ended", "name", name, "score", m.Score, "err", err)
	})
	return nil
}

func (s *Service) Progress() Progress {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.progress
}

func (s *Service) Cancel() {
	s.mu.Lock()
	stop := s.stop
	s.mu.Unlock()
	if stop != nil {
		stop()
		return
	}
	s.update(func(pr *Progress) { *pr = Progress{} })
}

func (s *Service) Save(p Profile) error {
	profiles := slices.DeleteFunc(s.Profiles(), func(q Profile) bool { return q.ID == p.ID })
	if err := s.SetProfiles(append(profiles, p)); err != nil {
		return err
	}
	return s.queue(change{Save: &p})
}

func (s *Service) Remove(id string) error {
	if err := s.SetProfiles(
		slices.DeleteFunc(s.Profiles(), func(q Profile) bool { return q.ID == id }),
	); err != nil {
		return err
	}
	return s.queue(change{Remove: id})
}

func (s *Service) update(change func(*Progress)) {
	s.mu.Lock()
	change(&s.progress)
	p := s.progress
	s.mu.Unlock()
	s.Changed.Emit(p)
}
