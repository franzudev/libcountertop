package photos

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/franzudev/libcountertop/pkg/media/photoprism"
	"github.com/franzudev/libcountertop/pkg/settings/schema"
)

func TestPhotoprismStatusGates(t *testing.T) {
	p := Photoprism()
	for _, c := range []struct {
		name string
		cfg  schema.Photoprism
		want Status
	}{
		{"off", schema.Photoprism{}, Off},
		{"no server", schema.Photoprism{Enabled: true}, Unset},
		{"connecting", schema.Photoprism{Enabled: true, Server: "https://x"}, Connecting},
	} {
		if got := p.Status(schema.Photos{Photoprism: c.cfg}); got != c.want {
			t.Errorf("%s: status %v, want %v", c.name, got, c.want)
		}
	}
}

func TestPhotoprismTopPhotosBuildADeckAtOpen(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/photos", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k3y" {
			http.Error(w, "{}", http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("offset") != "0" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		_, _ = w.Write([]byte(`[
			{"UID":"p1","Type":"image","TakenAt":"2020-05-01T10:00:00Z"},
			{"UID":"p2","Type":"video"},
			{"UID":"p3","Type":"image"}
		]`))
	})
	mux.HandleFunc("GET /api/v1/photos/{uid}/dl", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("jpeg data"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	s, _, err := Photoprism().Open(context.Background(), schema.Photos{
		Photoprism: schema.Photoprism{Server: srv.URL, Key: "k3y"},
	})
	if err != nil {
		t.Fatal(err)
	}
	front, back, err := s.Draw(context.Background(), time.Now(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(front) != 0 || len(back) != 2 {
		t.Fatalf("draw %d front, %d back; want 0 and 2 images", len(front), len(back))
	}
	ids := []string{back[0].ID, back[1].ID}
	if ids[0] != "p1" && ids[0] != "p3" {
		t.Errorf("first draw %v", ids)
	}
	if data, err := s.Fetch(context.Background(), ids[0]); err != nil || len(data) == 0 {
		t.Errorf("fetch %q: %d bytes, %v", ids[0], len(data), err)
	}
}

func TestPhotoprismAlbumNamesResolve(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/albums", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"UID":"aq1","Type":"album","Title":"Kitchen"}]`))
	})
	mux.HandleFunc("GET /api/v1/photos", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("album") != "aq1" {
			http.Error(w, "album", http.StatusBadRequest)
			return
		}
		if r.URL.Query().Get("offset") != "0" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		_, _ = w.Write([]byte(`[{"UID":"p9","Type":"image"}]`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	s, _, err := Photoprism().Open(context.Background(), schema.Photos{
		Photoprism: schema.Photoprism{Server: srv.URL, Key: "k", Albums: "kitchen, Missing"},
	})
	if !errors.As(err, new(*MissingError)) || err.(*MissingError).Name != "Missing" {
		t.Fatalf("open error %v, want a MissingError for Missing", err)
	}

	s, _, err = Photoprism().Open(context.Background(), schema.Photos{
		Photoprism: schema.Photoprism{Server: srv.URL, Key: "k", Albums: "Kitchen"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, back, err := s.Draw(context.Background(), time.Now(), true)
	if err != nil || len(back) != 1 || back[0].ID != "p9" {
		t.Fatalf("draw %v %v", back, err)
	}
}

func TestPhotoprismBadKeyAndGonePhotos(t *testing.T) {
	mux := http.NewServeMux()
	auth := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer k3y" {
				http.Error(w, "{}", http.StatusUnauthorized)
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("GET /api/v1/photos", auth(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"UID":"p1","Type":"image"}]`))
	}))
	mux.HandleFunc("GET /api/v1/photos/{uid}/dl", auth(func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("uid") == "p1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte("jpeg"))
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	if _, _, err := Photoprism().Open(context.Background(), schema.Photos{
		Photoprism: schema.Photoprism{Enabled: true, Server: srv.URL, Key: "nope"},
	}); !errors.Is(err, photoprism.ErrKey) {
		t.Fatalf("open with a refused key: %v, want ErrKey", err)
	}

	s, _, err := Photoprism().Open(context.Background(), schema.Photos{
		Photoprism: schema.Photoprism{Server: srv.URL, Key: "k3y"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Fetch(context.Background(), "p1"); !errors.Is(err, ErrGone) {
		t.Errorf("fetch a gone photo: %v, want ErrGone", err)
	}
}

func TestPhotoprismSameComparesItsSection(t *testing.T) {
	a := schema.Photos{Photoprism: schema.Photoprism{Albums: "Kitchen"}}
	b := a
	if !Photoprism().Same(a, b) {
		t.Error("the same config differs")
	}
	b.Photoprism.Server = "https://other"
	if Photoprism().Same(a, b) {
		t.Error("a changed server reads as the same")
	}
	c := a
	c.Immich.Server = "https://other"
	if !Photoprism().Same(a, c) {
		t.Error("a change in another section reads as ours")
	}
}
