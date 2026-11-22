package voicematchview

import (
	"testing"

	"github.com/franzudev/libcountertop/pkg/assistant/voicematch"
)

func TestEveryStageSaysSomething(t *testing.T) {
	for _, st := range []voicematch.Stage{voicematch.Prompt, voicematch.Heard, voicematch.Missed} {
		if statusText(voicematch.Step{Stage: st}) == " " {
			t.Errorf("stage %d has no status line", st)
		}
	}
}
