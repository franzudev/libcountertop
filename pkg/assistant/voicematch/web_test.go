package voicematch

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ygelfand/libcountertop/pkg/homeassistant/people"
)

func TestThePageListsVoicesAndPeople(t *testing.T) {
	s := newService(t)
	_ = s.SetProfiles([]Profile{
		{
			ID:        "person.ann",
			Name:      "Ann",
			Person:    "person.ann",
			Threshold: Default,
			Prints:    [][]float32{{1, 0}},
		},
	})
	h := s.Handler(func(context.Context) ([]people.Person, error) {
		return []people.Person{{ID: "person.ann", Name: "Ann"}, {ID: "person.bo", Name: "Bo"}}, nil
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	var got page
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Voices) != 1 || got.Voices[0].Person != "person.ann" || len(got.People) != 2 ||
		got.Enrolment["stage"] != "idle" {
		t.Errorf("page %+v", got)
	}
}

func TestThePageStillListsVoicesWithoutHomeAssistant(t *testing.T) {
	h := newService(t).Handler(func(context.Context) ([]people.Person, error) {
		return nil, errors.New("not connected")
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	var got page
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || got.PeopleError == "" || got.People == nil ||
		got.Voices == nil {
		t.Errorf("answer %d %+v", rec.Code, got)
	}
}

func TestThePageSetsAThresholdAndRemovesAVoice(t *testing.T) {
	s := newService(t)
	_ = s.SetProfiles([]Profile{
		{ID: "ann", Name: "Ann", Threshold: Default, Prints: [][]float32{{1, 0}}},
	})
	h := s.Handler(func(context.Context) ([]people.Person, error) { return nil, nil })
	for _, c := range []struct {
		body string
		want int
	}{
		{`{"threshold":0.42}`, http.StatusNoContent},
		{`{"threshold":0.9}`, http.StatusBadRequest},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(
			rec,
			httptest.NewRequest(http.MethodPut, "/voices/ann", strings.NewReader(c.body)),
		)
		if rec.Code != c.want {
			t.Errorf("threshold %s answered %d, want %d", c.body, rec.Code, c.want)
		}
	}
	if got := s.Profiles()[0].Threshold; got != 0.42 {
		t.Errorf("threshold %v, want 0.42", got)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/voices/ann", nil))
	if rec.Code != http.StatusNoContent || len(s.Profiles()) != 0 {
		t.Errorf("remove answered %d, profiles %+v", rec.Code, s.Profiles())
	}
}

func TestEnrollingFromThePageNeedsAName(t *testing.T) {
	h := newService(t).Handler(func(context.Context) ([]people.Person, error) { return nil, nil })
	rec := httptest.NewRecorder()
	h.ServeHTTP(
		rec,
		httptest.NewRequest(http.MethodPost, "/enroll", strings.NewReader(`{"name":" "}`)),
	)
	if rec.Code != http.StatusConflict {
		t.Errorf("an empty name answered %d, want %d", rec.Code, http.StatusConflict)
	}
}
