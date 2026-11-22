package photos

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	"github.com/franzudev/libcountertop/pkg/media/photoprism"
	"github.com/franzudev/libcountertop/pkg/say"
	"github.com/franzudev/libcountertop/pkg/settings/schema"
)

type photoprismSource struct{}

func Photoprism() Provider { return photoprismSource{} }

func (photoprismSource) Key() string { return "photoprism" }

func (photoprismSource) Name() string { return say.T("photos.photoprism") }

func (photoprismSource) Status(cfg schema.Photos) Status {
	switch {
	case !cfg.Photoprism.Enabled:
		return Off
	case cfg.Photoprism.Server == "":
		return Unset
	}
	return Connecting
}

func (photoprismSource) Same(a, b schema.Photos) bool { return a.Photoprism == b.Photoprism }

func (photoprismSource) Open(ctx context.Context, cfg schema.Photos) (Session, string, error) {
	c := photoprism.New(cfg.Photoprism.Server, cfg.Photoprism.Key)
	uids, err := collect(ctx, c, cfg.Photoprism)
	if err != nil {
		return nil, "", err
	}
	return &photoprismSession{c: c, uids: uids}, "", nil
}

type photoprismSession struct {
	c    *photoprism.Client
	uids []string
}

// Draw returns what it has: the session built its random order when it opened,
// the library keeps asking for more until the deck is empty, then it reopens.
func (s *photoprismSession) Draw(
	ctx context.Context,
	now time.Time,
	low bool,
) ([]Found, []Found, error) {
	if !low {
		return nil, nil, nil
	}
	out := make([]Found, len(s.uids))
	for i, uid := range s.uids {
		out[i] = Found{ID: uid}
	}
	return nil, out, nil
}

func (s *photoprismSession) Fetch(ctx context.Context, id string) ([]byte, error) {
	data, err := s.c.Picture(ctx, id)
	if errors.Is(err, photoprism.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", ErrGone, err)
	}
	return data, err
}

// match resolves wanted names to IDs against a list of items, matching an
// item's ID exactly or any of its names case-insensitively; an unknown name
// yields a MissingError. This was immich.go's helper, kept for photoprism.go's
// collect.
func match[T any](wants []string, all []T, of func(T) (string, []string)) ([]string, error) {
	var ids []string
	for _, want := range wants {
		found := ""
		for _, item := range all {
			id, names := of(item)
			if id == want || slices.ContainsFunc(names, func(n string) bool {
				return strings.EqualFold(n, want)
			}) {
				found = id
				break
			}
		}
		if found == "" {
			return nil, &MissingError{Name: want}
		}
		ids = append(ids, found)
	}
	return ids, nil
}

// collect resolves the wanted album names to UIDs and gathers their photos
// into one shuffled list; an empty album list fetches every indexed photo.
// Everything is read at open time only.
func collect(ctx context.Context, c *photoprism.Client, cfg schema.Photoprism) ([]string, error) {
	var uids []string
	if names := schema.Names(cfg.Albums); len(names) > 0 {
		all, err := c.Albums(ctx)
		if err != nil {
			return nil, err
		}
		ids, err := match(names, all, func(a photoprism.Album) (string, []string) {
			return a.UID, []string{a.Title}
		})
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			photos, err := c.Photos(ctx, id)
			if err != nil {
				return nil, err
			}
			for _, p := range photos {
				uids = append(uids, p.UID)
			}
			slog.Debug("photos: read a photoprism album", "album", id, "photos", len(photos))
		}
	} else {
		photos, err := c.Photos(ctx, "")
		if err != nil {
			return nil, err
		}
		for _, p := range photos {
			uids = append(uids, p.UID)
		}
		slog.Debug("photos: read every photoprism photo", "photos", len(uids))
	}
	rand.Shuffle(len(uids), func(i, j int) { uids[i], uids[j] = uids[j], uids[i] })
	return uids, nil
}
