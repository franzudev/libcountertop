package youtube

import (
	"context"
	"testing"
	"time"

	"github.com/ygelfand/libcountertop/pkg/media/playback"
)

func ids(vs []Video) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.ID
	}
	return out
}

func TestUpNextIsTheRestOfTheListThenRelated(t *testing.T) {
	p := NewPlayer(t.Context(), PlayerOptions{})
	p.list = []Video{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	p.index = 1
	p.related = []Video{{ID: "a"}, {ID: "x"}, {ID: "c"}, {ID: "y"}}
	got := ids(p.Upcoming())
	want := []string{"c", "x", "y"}
	if len(got) != len(want) {
		t.Fatalf("up next %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("up next %v, want %v", got, want)
		}
	}
	if prev, next := p.around(); !prev || !next {
		t.Errorf("around %v %v", prev, next)
	}
}

func TestTheLastVideoWithNothingRelatedHasNoNext(t *testing.T) {
	p := NewPlayer(t.Context(), PlayerOptions{})
	p.list = []Video{{ID: "a"}}
	if prev, next := p.around(); prev || next {
		t.Errorf("around %v %v", prev, next)
	}
	if _, ok := p.Playing(); ok {
		t.Error("an idle player says it is playing")
	}
}

func TestAPartlyWatchedVideoResumesWhereItWasLeft(t *testing.T) {
	ten := 10 * time.Minute
	for _, c := range []struct {
		watched int
		length  time.Duration
		want    time.Duration
	}{
		{watched: 40, length: ten, want: 4 * time.Minute},
		{watched: 0, length: ten},
		{watched: 96, length: ten},
		{watched: 100, length: ten},
		{watched: 40},
	} {
		if got := resumeAt(Video{Watched: c.watched, Length: c.length}); got != c.want {
			t.Errorf("%d%% of %v resumes at %v, want %v", c.watched, c.length, got, c.want)
		}
	}
}

type changes struct {
	playback.Output
	n int
}

func (c *changes) Changed(playback.Source) { c.n++ }

func TestAStreamFetchesMoreOnlyNearItsEnd(t *testing.T) {
	out := &changes{}
	p := NewPlayer(t.Context(), PlayerOptions{Output: out})
	s := &source{p: p}
	asked := 0
	p.cur, p.list = s, []Video{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}}
	p.more = func(context.Context) []Video {
		asked++
		return []Video{{ID: "d"}, {ID: "e"}}
	}
	p.relate(t.Context(), s)
	if asked != 0 {
		t.Fatalf("fetched more %d times at the start of a long list", asked)
	}
	p.index = 2
	p.relate(t.Context(), s)
	if got := ids(p.list); asked != 1 || len(got) != 5 || got[4] != "e" || out.n != 1 {
		t.Errorf("after nearing the end: asked %d, list %v, changes %d", asked, got, out.n)
	}
}

func TestTheSelectedAccountIsTheProfile(t *testing.T) {
	raw := `{"contents":[{"accountSectionListRenderer":{"contents":[{"accountItemSectionRenderer":{"contents":[
		{"accountItem":{"accountName":{"simpleText":"Brand"},"accountPhoto":{"thumbnails":[{"url":"https://yt3.example/b"}]}}},
		{"accountItem":{"accountName":{"runs":[{"text":"Person"}]},"isSelected":true,
			"accountPhoto":{"thumbnails":[{"url":"https://yt3.example/s"},{"url":"https://yt3.example/l"}]}}}]}}]}}]}`
	p, err := parseProfile([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if p != (Profile{Name: "Person", Photo: "https://yt3.example/l"}) {
		t.Errorf("profile %+v", p)
	}
	if _, err := parseProfile([]byte(`{"contents":{}}`)); err == nil {
		t.Error("a reply with no account gave a profile")
	}
}
