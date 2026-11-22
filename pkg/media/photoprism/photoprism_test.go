package photoprism_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/ygelfand/libcountertop/pkg/media/photoprism"
)

func fake(t *testing.T, key string) (*photoprism.Client, *photoprism.Client, *counts) {
	t.Helper()
	c := &counts{}
	mux := http.NewServeMux()
	auth := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer "+key {
				http.Error(w, `{"error":"not authorized"}`, http.StatusUnauthorized)
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("GET /api/v1/photos", auth(func(w http.ResponseWriter, r *http.Request) {
		c.photos++
		q := r.URL.Query()
		count, _ := strconv.Atoi(q.Get("count"))
		offset, _ := strconv.Atoi(q.Get("offset"))
		if q.Get("order") != "newest" || q.Get("type") != "image" {
			http.Error(w, "query", http.StatusBadRequest)
			return
		}
		album := q.Get("album")
		var out []map[string]any
		for i := offset; i < offset+count && i < 30; i++ {
			uid := strconv.Itoa(i)
			if album != "" && i >= 10 {
				break
			}
			kind := "image"
			if i%10 == 9 {
				kind = "video"
			}
			out = append(out, map[string]any{
				"UID":      "p" + uid,
				"Type":     kind,
				"TakenAt":  "2020-05-01T10:00:00Z",
				"FileName": uid + ".jpg",
			})
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
	mux.HandleFunc("GET /api/v1/albums", auth(func(w http.ResponseWriter, _ *http.Request) {
		c.albums++
		_, _ = w.Write([]byte(`[
			{"UID":"aq1bw1xxxxxx","Type":"album","Title":"Kitchen"},
			{"UID":"aq2bw1yyyyyy","Type":"album","Title":"Trips"},
			{"UID":"aq0bw1zzzzzz","Type":"moment","Title":"Basic"}
		]`))
	}))
	mux.HandleFunc("GET /api/v1/photos/{uid}/dl", auth(func(w http.ResponseWriter, r *http.Request) {
		c.downloads++
		if r.PathValue("uid") == "gone" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte("jpeg " + r.PathValue("uid")))
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return photoprism.New(srv.URL, key), photoprism.New(srv.URL, "wrong"), c
}

type counts struct {
	photos, albums, downloads int
}

func TestPhotosPagesAndKeepsOnlyImages(t *testing.T) {
	c, _, n := fake(t, "k3y")
	got, err := c.Photos(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 27 {
		t.Errorf("kept %d photos, want 27 of 30 minus the videos", len(got))
	}
	if got[0].UID != "p0" || got[0].Type != "image" {
		t.Errorf("first photo %+v", got[0])
	}
	if n.photos < 2 {
		t.Errorf("%d page requests, want paging", n.photos)
	}
}

func TestPhotosOfOneAlbumAskTheServerForIt(t *testing.T) {
	c, _, _ := fake(t, "k3y")
	got, err := c.Photos(context.Background(), "aq1bw1xxxxxx")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 9 {
		t.Errorf("kept %d photos, want the 9 images of the album", len(got))
	}
}

func TestAlbumsSkipTheCannedOnes(t *testing.T) {
	c, _, n := fake(t, "k3y")
	got, err := c.Albums(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Title != "Kitchen" || got[1].UID != "aq2bw1yyyyyy" {
		t.Errorf("albums %+v", got)
	}
	if n.albums != 1 {
		t.Errorf("%d album requests", n.albums)
	}
}

func TestPictureDownloadsTheOriginal(t *testing.T) {
	c, _, n := fake(t, "k3y")
	got, err := c.Picture(context.Background(), "p5")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "jpeg p5" {
		t.Errorf("got %q", got)
	}
	if n.downloads != 1 {
		t.Errorf("%d downloads", n.downloads)
	}
}

func TestARefusedKeySaysSo(t *testing.T) {
	_, bad, _ := fake(t, "k3y")
	for _, call := range []func() error{
		func() error { _, err := bad.Photos(context.Background(), ""); return err },
		func() error { _, err := bad.Albums(context.Background()); return err },
		func() error { _, err := bad.Picture(context.Background(), "p1"); return err },
	} {
		if err := call(); !errors.Is(err, photoprism.ErrKey) {
			t.Errorf("got %v, want ErrKey", err)
		}
	}
}

func TestAGonePhotoSaysSo(t *testing.T) {
	c, _, _ := fake(t, "k3y")
	if _, err := c.Picture(context.Background(), "gone"); !errors.Is(err, photoprism.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestNoServerSaysSo(t *testing.T) {
	c := photoprism.New("", "k3y")
	if _, err := c.Photos(context.Background(), ""); !errors.Is(err, photoprism.ErrNoServer) {
		t.Errorf("got %v, want ErrNoServer", err)
	}
}
