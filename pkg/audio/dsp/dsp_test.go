package dsp

import (
	"math"
	"testing"
)

func TestABiquadMatchesItsDirectForm(t *testing.T) {
	c := Coefficients{B0: 0.3, B1: -0.2, B2: 0.1, A1: -0.5, A2: 0.25}
	f := NewBiquad(c)
	var x1, x2, y1, y2 float64
	for i := range 200 {
		x := math.Sin(float64(i) * 0.3)
		y := c.B0*x + c.B1*x1 + c.B2*x2 - c.A1*y1 - c.A2*y2
		x2, x1, y2, y1 = x1, x, y1, y
		if got := f.Step(x); math.Abs(got-y) > 1e-12 {
			t.Fatalf("sample %d: %v, want %v", i, got, y)
		}
	}
}

func TestACascadeOfPassThroughsIsIdentity(t *testing.T) {
	c := NewCascade(Coefficients{B0: 1}, Coefficients{B0: 1})
	for _, x := range []float64{0.25, -0.5, 1} {
		if got := c.Step(x); got != x {
			t.Fatalf("%v came out as %v", x, got)
		}
	}
}
