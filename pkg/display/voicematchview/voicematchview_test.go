package voicematchview

import (
	"testing"

	"github.com/ygelfand/libcountertop/pkg/assistant/voicematch"
)

func TestTheWakeLabelLosesItsAssistantName(t *testing.T) {
	for label, want := range map[string]string{
		"Gemini: Hey Google": "Hey Google",
		"Alexa":              "Alexa",
		"":                   "",
	} {
		if got := spoken(label); got != want {
			t.Errorf("spoken(%q) = %q, want %q", label, got, want)
		}
	}
}

func TestATypedNameBecomesAnID(t *testing.T) {
	if got := slug("  Mary  Ann "); got != "mary_ann" {
		t.Errorf("slug = %q, want mary_ann", got)
	}
}

func TestEveryStageSaysSomething(t *testing.T) {
	for _, st := range []voicematch.Stage{voicematch.Prompt, voicematch.Heard, voicematch.Missed} {
		if statusText(voicematch.Step{Stage: st}) == " " {
			t.Errorf("stage %d has no status line", st)
		}
	}
}
