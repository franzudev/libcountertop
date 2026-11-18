package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

type reelEndpoint struct {
	Params       string `json:"params"`
	PlayerParams string `json:"playerParams"`
	InputType    string `json:"inputType"`
}

type reelSequence struct {
	Entries []struct {
		Command struct {
			Reel struct {
				VideoID   string `json:"videoId"`
				Thumbnail struct {
					Thumbnails []Picture `json:"thumbnails"`
				} `json:"thumbnail"`
			} `json:"reelWatchEndpoint"`
		} `json:"command"`
	} `json:"entries"`
	Continuation string `json:"continuation"`
}

func (s *Session) Shorts(ctx context.Context) (Page, error) {
	raw, err := s.post(ctx, "guide", map[string]any{})
	if err != nil {
		return Page{}, err
	}
	seed, err := parseShortsEntry(raw)
	if err != nil {
		return Page{}, err
	}
	raw, err = s.post(ctx, "reel/reel_item_watch", map[string]any{
		"params":                seed.Params,
		"inputType":             seed.InputType,
		"playerRequest":         map[string]any{"params": seed.PlayerParams},
		"disablePlayerResponse": true,
	})
	if err != nil {
		return Page{}, err
	}
	var item struct {
		Sequence string `json:"sequenceContinuation"`
	}
	if err := json.Unmarshal(raw, &item); err != nil {
		return Page{}, fmt.Errorf("youtube: shorts: %w", err)
	}
	if item.Sequence == "" {
		return Page{}, errors.New("youtube: shorts: no sequence in the reply")
	}
	return s.MoreShorts(ctx, item.Sequence)
}

func (s *Session) MoreShorts(ctx context.Context, token string) (Page, error) {
	raw, err := s.post(ctx, "reel/reel_watch_sequence", map[string]any{"sequenceParams": token})
	if err != nil {
		return Page{}, err
	}
	return parseReels(raw)
}

func Describe(ctx context.Context, hc *http.Client, id string) (Video, error) {
	page := base + "/shorts/" + url.PathEscape(id)
	raw, err := get(ctx, hc, base+"/oembed?format=json&url="+url.QueryEscape(page))
	if err != nil {
		return Video{}, fmt.Errorf("youtube: describe: %w", err)
	}
	var got struct {
		Title  string `json:"title"`
		Author string `json:"author_name"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		return Video{}, fmt.Errorf("youtube: describe: %w", err)
	}
	return Video{ID: id, Title: got.Title, Channel: got.Author, Kind: Short}, nil
}

func parseShortsEntry(raw []byte) (reelEndpoint, error) {
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return reelEndpoint{}, fmt.Errorf("youtube: guide: %w", err)
	}
	found, ok := findKey(root["items"], "reelWatchEndpoint")
	if !ok {
		return reelEndpoint{}, errors.New("youtube: guide: no shorts entry")
	}
	var e reelEndpoint
	if !decode(found, &e) {
		return reelEndpoint{}, errors.New("youtube: guide: unreadable shorts entry")
	}
	e.Params = unescaped(e.Params)
	e.PlayerParams = unescaped(e.PlayerParams)
	return e, nil
}

func parseReels(raw []byte) (Page, error) {
	var seq reelSequence
	if err := json.Unmarshal(raw, &seq); err != nil {
		return Page{}, fmt.Errorf("youtube: shorts: %w", err)
	}
	var shelf Shelf
	for _, e := range seq.Entries {
		r := e.Command.Reel
		if r.VideoID == "" {
			continue
		}
		shelf.Videos = append(shelf.Videos, Video{
			ID:        r.VideoID,
			Kind:      Short,
			Thumbnail: pickThumbnail(r.Thumbnail.Thumbnails, true),
		})
	}
	p := Page{More: seq.Continuation}
	if len(shelf.Videos) > 0 {
		p.Shelves = []Shelf{shelf}
	}
	return p, nil
}

func findKey(n any, key string) (any, bool) {
	switch v := n.(type) {
	case []any:
		for _, c := range v {
			if got, ok := findKey(c, key); ok {
				return got, true
			}
		}
	case map[string]any:
		if got, ok := v[key]; ok {
			return got, true
		}
		for _, k := range sortedKeys(v) {
			if got, ok := findKey(v[k], key); ok {
				return got, true
			}
		}
	}
	return nil, false
}

func unescaped(s string) string {
	if u, err := url.QueryUnescape(s); err == nil {
		return u
	}
	return s
}
