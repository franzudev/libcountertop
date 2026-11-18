package youtube

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type fakeGoogle struct {
	pending   atomic.Int32
	refresh   atomic.Int32
	answer    string
	browsed   atomic.Value
	searched  atomic.Value
	reeled    atomic.Value
	sequenced atomic.Value
	bearer    atomic.Value
	deviceID  atomic.Value
}

func reply(t *testing.T, w io.Writer, body string) {
	if _, err := io.WriteString(w, body); err != nil {
		t.Error(err)
	}
}

func readJSON(t *testing.T, r *http.Request, v any) {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		t.Error(err)
	}
}

func (f *fakeGoogle) serve(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tv", func(w http.ResponseWriter, r *http.Request) {
		reply(t,
			w,
			`<html><script id="base-js" src="/s/tv/base.js" nonce="x"></script></html>`,
		)
	})
	mux.HandleFunc("GET /s/tv/base.js", func(w http.ResponseWriter, r *http.Request) {
		reply(t, w, `var a={clientId:"tv-client.apps",zz:"tv-secret"};`)
	})
	mux.HandleFunc("POST /o/oauth2/device/code", func(w http.ResponseWriter, r *http.Request) {
		var got map[string]string
		readJSON(t, r, &got)
		f.deviceID.Store(got["device_id"])
		if got["client_id"] != "tv-client.apps" || got["device_model"] != deviceModel {
			t.Errorf("device code asked with %v", got)
		}
		reply(
			t,
			w,
			`{"device_code":"dev-1","user_code":"ABC-DEF-GHI","verification_url":"https://www.google.com/device","expires_in":1800,"interval":5}`,
		)
	})
	mux.HandleFunc("POST /o/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		var got map[string]string
		readJSON(t, r, &got)
		if got["client_secret"] != "tv-secret" {
			t.Errorf("token asked with %v", got)
		}
		switch got["grant_type"] {
		case deviceGrant:
			if f.pending.Add(-1) >= 0 {
				w.WriteHeader(http.StatusPreconditionRequired)
				reply(t, w, `{"error":"authorization_pending"}`)
				return
			}
			if f.answer != "" {
				w.WriteHeader(http.StatusForbidden)
				reply(t, w, `{"error":"`+f.answer+`"}`)
				return
			}
			reply(t,
				w,
				`{"access_token":"access-1","refresh_token":"refresh-1","expires_in":3600}`,
			)
		case "refresh_token":
			if got["refresh_token"] != "refresh-1" {
				t.Errorf("refreshed with %q", got["refresh_token"])
			}
			f.refresh.Add(1)
			reply(t, w, `{"access_token":"access-2","expires_in":3600}`)
		}
	})
	mux.HandleFunc("POST /youtubei/v1/browse", func(w http.ResponseWriter, r *http.Request) {
		f.bearer.Store(r.Header.Get("Authorization"))
		var got map[string]any
		readJSON(t, r, &got)
		f.browsed.Store(got)
		reply(t, w, `{"contents":{"gridRenderer":{"items":[`+
			tileJSON(
				"aaaaaaaaaaa",
				"One",
				"Chan",
				"1 view",
				"1:00",
				"TILE_CONTENT_TYPE_VIDEO",
				"",
			)+`]}}}`)
	})
	mux.HandleFunc("POST /youtubei/v1/search", func(w http.ResponseWriter, r *http.Request) {
		var got map[string]any
		readJSON(t, r, &got)
		f.searched.Store(got)
		reply(
			t,
			w,
			`{"contents":{"sectionListRenderer":{"contents":[{"itemSectionRenderer":{"contents":[`+
				tileJSON(
					"bbbbbbbbbbb",
					"Found",
					"Chan",
					"2 views",
					"2:00",
					"TILE_CONTENT_TYPE_VIDEO",
					"",
				)+
				`]}}],"continuations":[{"nextContinuationData":{"continuation":"results2"}}]}}}`,
		)
	})
	mux.HandleFunc("POST /youtubei/v1/guide", func(w http.ResponseWriter, r *http.Request) {
		reply(t, w, `{"items":[{"guideSectionRenderer":{"items":[`+
			`{"guideEntryRenderer":{"navigationEndpoint":{"browseEndpoint":{"browseId":"FEtopics"}}}},`+
			`{"guideEntryRenderer":{"navigationEndpoint":{"reelWatchEndpoint":`+
			`{"playerParams":"pp1","params":"CA8%3D","inputType":"REEL_WATCH_INPUT_TYPE_SEEDLESS"}}}}]}}],`+
			`"startupBehaviours":[{"launchToShorts":{"resumeToShorts":{"reelWatchEndpoint":{"params":"wrong"}}}}]}`)
	})
	mux.HandleFunc(
		"POST /youtubei/v1/reel/reel_item_watch",
		func(w http.ResponseWriter, r *http.Request) {
			var got map[string]any
			readJSON(t, r, &got)
			f.reeled.Store(got)
			reply(t, w, `{"sequenceContinuation":"seq1"}`)
		},
	)
	mux.HandleFunc(
		"POST /youtubei/v1/reel/reel_watch_sequence",
		func(w http.ResponseWriter, r *http.Request) {
			var got map[string]any
			readJSON(t, r, &got)
			f.sequenced.Store(got)
			reply(t, w, `{"entries":[`+
				`{"command":{"reelWatchEndpoint":{"videoId":"sssssssssss","thumbnail":{"thumbnails":`+
				`[{"url":"https://i.example/s.jpg","width":1080,"height":1920}]}}}},`+
				`{"command":{"reelWatchEndpoint":{}}}],"continuation":"seq2"}`)
		},
	)
	mux.HandleFunc("GET /oembed", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("url") != base+"/shorts/sssssssssss" {
			http.NotFound(w, r)
			return
		}
		reply(t, w, `{"title":"A short","author_name":"Chan","type":"video"}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	old := base
	base = srv.URL
	t.Cleanup(func() { base = old })
}

func TestASignInCodeBecomesASession(t *testing.T) {
	f := &fakeGoogle{}
	f.pending.Store(2)
	f.serve(t)
	ctx := t.Context()

	c, err := TVClient(ctx, nil)
	if err != nil || c != (Client{ID: "tv-client.apps", Secret: "tv-secret"}) {
		t.Fatalf("client %+v, %v", c, err)
	}
	code, err := Ask(ctx, nil, c, "panel-1")
	if err != nil {
		t.Fatal(err)
	}
	if code.User != "ABC-DEF-GHI" || code.URL != "https://www.google.com/device" ||
		code.Interval != 5*time.Second {
		t.Errorf("code %+v", code)
	}
	if id := f.deviceID.Load(); id != "panel-1" {
		t.Errorf("device id %v", id)
	}
	code.Interval = time.Millisecond
	tok, err := Await(ctx, nil, c, code)
	if err != nil {
		t.Fatal(err)
	}
	if tok.Access != "access-1" || tok.Refresh != "refresh-1" {
		t.Errorf("token %+v", tok)
	}

	s := SignedIn(nil, c, tok)
	p, err := s.Browse(ctx, History)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Shelves) != 1 || p.Shelves[0].Videos[0].ID != "aaaaaaaaaaa" {
		t.Errorf("page %+v", p)
	}
	if b := f.bearer.Load(); b != "Bearer access-1" {
		t.Errorf("browsed with %v", b)
	}
	sent := f.browsed.Load().(map[string]any)
	client := sent["context"].(map[string]any)["client"].(map[string]any)
	if sent["browseId"] != History || client["clientName"] != tvClientName {
		t.Errorf("browse body %v", sent)
	}
	if f.refresh.Load() != 0 {
		t.Error("a fresh token was refreshed")
	}
}

func TestASavedAccountRefreshesBeforeBrowsing(t *testing.T) {
	f := &fakeGoogle{}
	f.serve(t)
	s := NewSession(
		nil,
		Account{Client: Client{ID: "tv-client.apps", Secret: "tv-secret"}, Refresh: "refresh-1"},
	)
	if _, err := s.More(t.Context(), "next"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Browse(t.Context(), Home); err != nil {
		t.Fatal(err)
	}
	if n := f.refresh.Load(); n != 1 {
		t.Errorf("refreshed %d times for two pages", n)
	}
	if b := f.bearer.Load(); b != "Bearer access-2" {
		t.Errorf("browsed with %v", b)
	}
	if got := s.Account().Refresh; got != "refresh-1" {
		t.Errorf("kept refresh token %q", got)
	}
}

func TestSearchAsksTheSearchEndpoint(t *testing.T) {
	f := &fakeGoogle{}
	f.serve(t)
	s := SignedIn(nil, Client{ID: "tv-client.apps", Secret: "tv-secret"},
		Token{Access: "access-1", Refresh: "refresh-1", Expiry: time.Now().Add(time.Hour)})
	p, err := s.Search(t.Context(), "pizza")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Shelves) != 1 || p.Shelves[0].Videos[0].ID != "bbbbbbbbbbb" || p.More != "results2" {
		t.Errorf("page %+v", p)
	}
	if sent := f.searched.Load().(map[string]any); sent["query"] != "pizza" {
		t.Errorf("search body %v", sent)
	}
	if _, err := s.SearchMore(t.Context(), "results2"); err != nil {
		t.Fatal(err)
	}
	if sent := f.searched.Load().(map[string]any); sent["continuation"] != "results2" {
		t.Errorf("next results body %v", sent)
	}
}

func TestShortsStartFromTheGuidesShortsEntry(t *testing.T) {
	f := &fakeGoogle{}
	f.serve(t)
	s := SignedIn(nil, Client{ID: "tv-client.apps", Secret: "tv-secret"},
		Token{Access: "access-1", Refresh: "refresh-1", Expiry: time.Now().Add(time.Hour)})
	p, err := s.Shorts(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	item := f.reeled.Load().(map[string]any)
	player, _ := item["playerRequest"].(map[string]any)
	if item["params"] != "CA8=" || item["inputType"] != "REEL_WATCH_INPUT_TYPE_SEEDLESS" ||
		player["params"] != "pp1" {
		t.Errorf("reel item body %v", item)
	}
	if sent := f.sequenced.Load().(map[string]any); sent["sequenceParams"] != "seq1" {
		t.Errorf("sequence body %v", sent)
	}
	want := Video{
		ID:        "sssssssssss",
		Kind:      Short,
		Thumbnail: Picture{URL: "https://i.example/s.jpg", Width: 1080, Height: 1920},
	}
	if len(p.Shelves) != 1 || len(p.Shelves[0].Videos) != 1 || p.Shelves[0].Videos[0] != want ||
		p.More != "seq2" {
		t.Errorf("shorts page %+v", p)
	}
	if _, err := s.MoreShorts(t.Context(), "seq2"); err != nil {
		t.Fatal(err)
	}
	if sent := f.sequenced.Load().(map[string]any); sent["sequenceParams"] != "seq2" {
		t.Errorf("next sequence body %v", sent)
	}
}

func TestAShortIsDescribedByItsOEmbed(t *testing.T) {
	f := &fakeGoogle{}
	f.serve(t)
	v, err := Describe(t.Context(), nil, "sssssssssss")
	if err != nil {
		t.Fatal(err)
	}
	if v != (Video{ID: "sssssssssss", Title: "A short", Channel: "Chan", Kind: Short}) {
		t.Errorf("described %+v", v)
	}
	if _, err := Describe(t.Context(), nil, "unknown0000"); err == nil {
		t.Error("an unknown short was described")
	}
}

func TestARefusedSignInSaysSo(t *testing.T) {
	f := &fakeGoogle{answer: "access_denied"}
	f.serve(t)
	c := Client{ID: "tv-client.apps", Secret: "tv-secret"}
	code, err := Ask(t.Context(), nil, c, "panel-1")
	if err != nil {
		t.Fatal(err)
	}
	code.Interval = time.Millisecond
	if _, err := Await(t.Context(), nil, c, code); !errors.Is(err, ErrDenied) {
		t.Errorf("got %v, want refused", err)
	}
}

func TestAnUnansweredCodeExpires(t *testing.T) {
	f := &fakeGoogle{}
	f.pending.Store(1 << 30)
	f.serve(t)
	c := Client{ID: "tv-client.apps", Secret: "tv-secret"}
	code, err := Ask(t.Context(), nil, c, "panel-1")
	if err != nil {
		t.Fatal(err)
	}
	code.Interval, code.Expires = time.Millisecond, time.Now().Add(50*time.Millisecond)
	if _, err := Await(t.Context(), nil, c, code); !errors.Is(err, ErrExpired) {
		t.Errorf("got %v, want expired", err)
	}
}
