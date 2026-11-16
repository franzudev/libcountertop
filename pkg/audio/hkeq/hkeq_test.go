package hkeq

import (
	"encoding/binary"
	"math"
	"os"
	"testing"
)

const fixtureFrames = 28800

func fixture(i int) float64 {
	t := float64(i) / Rate
	a := math.Pow(10, -36.0/20)
	if i >= fixtureFrames/2 {
		a = math.Pow(10, -6.0/20)
	}
	return a / 2 * (math.Sin(2*math.Pi*100*t) + math.Sin(2*math.Pi*1000*t) + math.Sin(2*math.Pi*5000*t))
}

func load(t *testing.T) Tuning {
	t.Helper()
	raw, err := os.ReadFile("testdata/defaultEQ.txt")
	if err != nil {
		t.Fatal(err)
	}
	tuning, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return tuning
}

func TestTheFileDecodesToHarmansLayout(t *testing.T) {
	tuning := load(t)
	if tuning.PreDB != -14 || tuning.PostDB != 12 || tuning.Lookahead != 400 {
		t.Fatalf("gains %v/%v dB, lookahead %d", tuning.PreDB, tuning.PostDB, tuning.Lookahead)
	}
	if len(tuning.EQ) != eqSections || len(tuning.Tail) != tailSections {
		t.Fatalf("%d and %d sections", len(tuning.EQ), len(tuning.Tail))
	}
	if tuning.Low.Hold != 110 || tuning.High.Hold != 110 || tuning.Low.Slope != 705 ||
		tuning.High.Slope != 560 {
		t.Fatalf("bands %+v and %+v", tuning.Low, tuning.High)
	}
}

func TestTheChainTracksHarmansOwnOutput(t *testing.T) {
	ref, err := os.ReadFile("testdata/harman.pcm")
	if err != nil {
		t.Fatal(err)
	}
	c := load(t).Chain()
	var errEnergy, refEnergy float64
	for i := range fixtureFrames {
		want := float64(int16(binary.LittleEndian.Uint16(ref[i*2:]))) / 32768
		got := float64(float32(c.Step(float64(float32(fixture(i))))))
		errEnergy += (got - want) * (got - want)
		refEnergy += want * want
	}
	db := 10 * math.Log10(errEnergy/refEnergy)
	t.Logf("error %.2f dB relative to Harman's output", db)
	if db > -28 {
		t.Fatalf("error %.2f dB relative to Harman's output, want below -28", db)
	}
}

func TestAShortFileIsRefused(t *testing.T) {
	if _, err := Parse(make([]byte, 16)); err == nil {
		t.Fatal("a 16-byte file parsed")
	}
}
