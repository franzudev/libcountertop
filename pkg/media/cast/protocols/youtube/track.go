package youtube

import (
	"context"
	"io"
	"sync/atomic"
	"time"

	"github.com/franzudev/libcountertop/pkg/media/playback"
	"github.com/franzudev/libcountertop/pkg/media/sponsorblock"
	"github.com/franzudev/libcountertop/pkg/media/youtube"
)

type track struct {
	*youtube.Track

	station *station
	stalled atomic.Bool
}

var (
	_ playback.Source   = (*track)(nil)
	_ playback.Pictures = (*track)(nil)
	_ playback.Seeker   = (*track)(nil)
	_ playback.Marked   = (*track)(nil)
	_ playback.Live     = (*track)(nil)
)

func openTrack(
	ctx context.Context,
	r *youtube.Resolver,
	st *station,
	id string,
	at time.Duration,
) (*track, error) {
	t := &track{station: st}
	inner, err := youtube.Open(ctx, r, id, youtube.Options{
		At:       at,
		Skips:    func(ctx context.Context) []sponsorblock.Segment { return st.segments(ctx, id) },
		Finished: func() { st.finished(t) },
		Skip:     func(seg sponsorblock.Segment) { st.skipWhenHeard(t, seg) },
		Seek:     st.seek,
	})
	if err != nil {
		return nil, err
	}
	t.Track = inner
	return t, nil
}

func (t *track) Pictured() bool {
	return t.station != nil && t.station.pictured() && t.Track.Pictured()
}

func (t *track) Pictures(ctx context.Context) (io.ReadCloser, time.Duration, error) {
	if !t.Pictured() {
		return nil, 0, nil
	}
	return t.Track.Pictures(ctx)
}

func (t *track) Live(ctx context.Context) (playback.Picture, error) {
	if !t.Pictured() {
		return playback.Picture{}, nil
	}
	return t.Track.Live(ctx)
}

func (t *track) Logo() string {
	if t.station == nil {
		return ""
	}
	return t.station.logo()
}

func (t *track) Waiting(w bool) {
	if t.station == nil {
		return
	}
	t.stalled.Store(w)
	go t.tell()
}

func (t *track) tell() {
	if t.station.current() != t {
		return
	}
	if t.stalled.Load() {
		t.station.report(stateLoading)
		return
	}
	t.station.report(t.station.state())
}

func (t *track) Showing() playback.Showing {
	s := t.Track.Showing()
	if t.station != nil {
		s.HasPrevious, s.HasNext = t.station.around()
		s.Upcoming = t.station.coming()
	}
	return s
}

func (t *track) Label() string {
	if t.station == nil {
		return ""
	}
	return t.station.label()
}

func (t *track) Play()     { t.station.play() }
func (t *track) Pause()    { t.station.pause() }
func (t *track) Stop()     { t.station.stop() }
func (t *track) Next()     { t.station.step(1) }
func (t *track) Previous() { t.station.step(-1) }

func (s *station) pictured() bool {
	return s.env.Video != nil && s.theme == ThemeYouTube
}
