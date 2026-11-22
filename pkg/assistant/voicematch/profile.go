package voicematch

import (
	"fmt"
	"time"

	"github.com/franzudev/libcountertop/pkg/inference/voiceprint"
)

const (
	Loosest   = 0.30
	Default   = 0.50
	Strictest = 0.70
)

type Profile struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Person    string      `json:"person,omitempty"`
	Prints    [][]float32 `json:"prints"`
	Threshold float64     `json:"threshold"`
}

func (p Profile) Valid() error {
	if p.Threshold < Loosest || p.Threshold > Strictest {
		return fmt.Errorf(
			"voicematch: %s has threshold %.2f, want %.2f to %.2f",
			p.ID, p.Threshold, Loosest, Strictest,
		)
	}
	return nil
}

type Match struct {
	Profile Profile
	Score   float64
}

type Identified struct {
	At    time.Time
	Match Match
	Known bool
}

func (p Profile) Who() string {
	if p.Person != "" {
		return p.Person
	}
	return p.ID
}

func best(print []float32, profiles []Profile) (Match, bool) {
	var m Match
	found := false
	for _, p := range profiles {
		for _, take := range p.Prints {
			if s := voiceprint.Similarity(print, take); !found || s > m.Score {
				m, found = Match{Profile: p, Score: s}, true
			}
		}
	}
	return m, found
}
