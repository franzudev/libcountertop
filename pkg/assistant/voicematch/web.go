package voicematch

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ygelfand/libcountertop/pkg/homeassistant/people"
)

const (
	bodyMost   = 4 << 10
	peopleWait = 10 * time.Second
)

type voice struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Person    string  `json:"person"`
	Threshold float64 `json:"threshold"`
}

type limits struct {
	Loosest   float64 `json:"loosest"`
	Strictest float64 `json:"strictest"`
	Default   float64 `json:"default"`
}

type page struct {
	Limits      limits            `json:"limits"`
	Voices      []voice           `json:"voices"`
	People      []people.Person   `json:"people"`
	PeopleError string            `json:"people_error,omitempty"`
	Enrolment   map[string]string `json:"enrolment"`
}

func (s *Service) Handler(list func(context.Context) ([]people.Person, error)) http.Handler {
	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		p := page{
			Limits:    limits{Loosest: Loosest, Strictest: Strictest, Default: Default},
			Voices:    []voice{},
			People:    []people.Person{},
			Enrolment: s.Progress().fields(),
		}
		for _, v := range s.Profiles() {
			p.Voices = append(
				p.Voices,
				voice{ID: v.ID, Name: v.Name, Person: v.Person, Threshold: v.Threshold},
			)
		}
		ctx, cancel := context.WithTimeout(r.Context(), peopleWait)
		defer cancel()
		found, err := list(ctx)
		if err != nil {
			slog.Warn("listing people for the voice page failed", "err", err)
			p.PeopleError = err.Error()
		} else {
			p.People = found
		}
		send(w, p)
	})
	r.Post("/enroll", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name   string `json:"name"`
			Person string `json:"person"`
			Voice  string `json:"voice"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, bodyMost)).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		step, err := s.start(body.Name, body.Person, body.Voice)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		send(w, step)
	})
	r.Post("/cancel", func(w http.ResponseWriter, _ *http.Request) {
		s.Cancel()
		w.WriteHeader(http.StatusNoContent)
	})
	r.Delete("/voices/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Remove(chi.URLParam(r, "id")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	r.Put("/voices/{id}", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Threshold float64 `json:"threshold"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, bodyMost)).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id := chi.URLParam(r, "id")
		all := s.Profiles()
		i := slices.IndexFunc(all, func(p Profile) bool { return p.ID == id })
		if i < 0 {
			http.Error(w, "no such voice", http.StatusNotFound)
			return
		}
		p := all[i]
		p.Threshold = body.Threshold
		if err := p.Valid(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.Save(p); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	r.Post("/accept", func(w http.ResponseWriter, _ *http.Request) {
		if err := s.Accept(); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return r
}

func send(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("writing a voice page reply failed", "err", err)
	}
}
