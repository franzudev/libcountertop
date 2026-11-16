package voicematch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/ygelfand/libcountertop/pkg/runtime/safe"
)

const (
	exchange  = "echolocal.voices"
	reply     = "{{ response | to_json }}"
	syncLimit = 30 * time.Second
)

type Asker interface {
	Ask(
		ctx context.Context,
		action string,
		data, templates map[string]string,
		tmpl string,
	) (json.RawMessage, error)
}

type change struct {
	Save   *Profile `json:"save,omitempty"`
	Remove string   `json:"remove,omitempty"`
}

func (s *Service) Sync(ctx context.Context) error {
	if s.o.Home == nil {
		return nil
	}
	s.syncing.Lock()
	defer s.syncing.Unlock()

	s.mu.Lock()
	sent := len(s.outbox)
	raw, err := json.Marshal(append([]change{}, s.outbox...))
	s.mu.Unlock()
	if err != nil {
		return err
	}
	answer, err := s.o.Home.Ask(
		ctx,
		exchange,
		map[string]string{"changes": string(raw)},
		nil,
		reply,
	)
	if err != nil {
		return fmt.Errorf("voicematch: syncing voices: %w", err)
	}
	var got struct {
		Profiles []Profile `json:"profiles"`
	}
	if err := json.Unmarshal(answer, &got); err != nil {
		return fmt.Errorf("voicematch: the household's voices: %w", err)
	}
	if err := s.SetProfiles(got.Profiles); err != nil {
		return err
	}
	s.mu.Lock()
	s.outbox = s.outbox[sent:]
	err = s.keepOutbox()
	s.mu.Unlock()
	slog.Info("voices synced", "sent", sent, "voices", len(got.Profiles))
	return err
}

func (s *Service) Resync() {
	safe.Go("voice sync", func() {
		ctx, cancel := context.WithTimeout(context.Background(), syncLimit)
		defer cancel()
		if err := s.Sync(ctx); err != nil {
			slog.Warn("voices not synced", "err", err)
		}
	})
}

func (s *Service) queue(c change) error {
	s.mu.Lock()
	s.outbox = append(s.outbox, c)
	err := s.keepOutbox()
	s.mu.Unlock()
	if err != nil {
		return err
	}
	s.Resync()
	return nil
}

func (s *Service) loadOutbox() {
	raw, err := os.ReadFile(s.o.Outbox)
	switch {
	case errors.Is(err, os.ErrNotExist):
		for _, p := range s.profiles {
			s.outbox = append(s.outbox, change{Save: &p})
		}
		if err := s.keepOutbox(); err != nil {
			slog.Warn("voice outbox not written", "err", err)
		}
	case err != nil:
		slog.Warn("voice outbox unreadable", "path", s.o.Outbox, "err", err)
	default:
		if err := json.Unmarshal(raw, &s.outbox); err != nil {
			slog.Warn("voice outbox unreadable", "path", s.o.Outbox, "err", err)
		}
	}
}

func (s *Service) keepOutbox() error {
	if s.o.Outbox == "" {
		return nil
	}
	raw, err := json.Marshal(s.outbox)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.o.Outbox), 0o755); err != nil {
		return fmt.Errorf("voice outbox: %w", err)
	}
	tmp := s.o.Outbox + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("voice outbox: %w", err)
	}
	return os.Rename(tmp, s.o.Outbox)
}
