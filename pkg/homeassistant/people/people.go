package people

import (
	"context"
	"slices"
	"strings"

	ha "github.com/ygelfand/libcountertop/pkg/homeassistant"
)

type Person struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Picture string `json:"picture,omitempty"`
}

type Entities interface {
	Entities(ctx context.Context, f ha.Filter) ([]ha.Entity, error)
}

func List(ctx context.Context, e Entities) ([]Person, error) {
	entities, err := e.Entities(ctx, ha.Filter{
		Domains:    []string{"person"},
		Attributes: []string{"entity_picture"},
	})
	if err != nil {
		return nil, err
	}
	out := make([]Person, 0, len(entities))
	for _, en := range entities {
		out = append(
			out,
			Person{ID: en.ID, Name: en.Name, Picture: en.Attributes["entity_picture"]},
		)
	}
	slices.SortFunc(out, func(a, b Person) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out, nil
}
