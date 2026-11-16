package avatar

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"

	ha "github.com/ygelfand/libcountertop/pkg/homeassistant"
)

const pictureMost = 4 << 20

type Entities interface {
	Entities(ctx context.Context, f ha.Filter) ([]ha.Entity, error)
}

type Site interface{ URL() string }

type HomeAssistant struct {
	Entities Entities
	Site     Site
	Client   *http.Client
}

func (h HomeAssistant) People(ctx context.Context) ([]Person, error) {
	entities, err := h.Entities.Entities(ctx, ha.Filter{
		Domains:    []string{"person"},
		Attributes: []string{"entity_picture"},
	})
	if err != nil {
		return nil, err
	}
	out := make([]Person, 0, len(entities))
	for _, e := range entities {
		out = append(out, Person{ID: e.ID, Name: e.Name, Picture: e.Attributes["entity_picture"]})
	}
	return out, nil
}

func (h HomeAssistant) Picture(ctx context.Context, path string) (image.Image, error) {
	base := h.Site.URL()
	if base == "" {
		return nil, fmt.Errorf("avatar: no Home Assistant address for %s", path)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := h.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("avatar: %s answered %d", path, resp.StatusCode)
	}
	img, _, err := image.Decode(io.LimitReader(resp.Body, pictureMost))
	return img, err
}
