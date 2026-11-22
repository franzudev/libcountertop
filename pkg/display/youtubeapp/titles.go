package youtubeapp

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"github.com/franzudev/libcountertop/pkg/media/youtube"
	"github.com/franzudev/libcountertop/pkg/runtime/safe"
)

type titles struct {
	client *http.Client
	redraw func()

	mu  sync.Mutex
	got map[string]youtube.Video
}

func newTitles(client *http.Client, redraw func()) *titles {
	return &titles{client: client, redraw: redraw, got: map[string]youtube.Video{}}
}

func (t *titles) named(v youtube.Video) youtube.Video {
	if v.Title != "" {
		return v
	}
	t.mu.Lock()
	found, asked := t.got[v.ID]
	if !asked {
		t.got[v.ID] = youtube.Video{}
	}
	t.mu.Unlock()
	if asked {
		v.Title, v.Channel = found.Title, found.Channel
		return v
	}
	safe.Go("youtube title", func() { t.ask(v.ID) })
	return v
}

func (t *titles) ask(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), askTime)
	defer cancel()
	v, err := youtube.Describe(ctx, t.client, id)
	if err != nil {
		slog.Debug("youtube title", "video", id, "err", err)
		return
	}
	t.mu.Lock()
	t.got[id] = v
	t.mu.Unlock()
	t.redraw()
}
