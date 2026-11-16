package hkeq

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/ygelfand/libcountertop/pkg/audio/dsp"
)

const Rate = 48000

const (
	preWord      = 1
	eqWord       = 4
	eqSections   = 10
	lowWord      = 54
	highWord     = 61
	highPassWord = 68
	lowPassWord  = 73
	lookWord     = 78
	tailWord     = 79
	tailSections = 7
	postWord     = 114
	words        = postWord + 1
)

// Harman's gain smoother, BesselFirCoefs and BesselIirCoefs in libhk-audio-effect-lib.so.
var bessel = dsp.Coefficients{
	B1: 1721380.0 / (1 << 30),
	A1: -2072576217.0 / (1 << 30),
	A2: 1000556012.0 / (1 << 30),
}

type Band struct {
	Scale, Threshold, Floor, Release, Slope float64
	Hold                                    int
}

type Tuning struct {
	PreDB, PostDB     float64
	EQ, Tail          []dsp.Coefficients
	HighPass, LowPass dsp.Coefficients
	Low, High         Band
	Lookahead         int
}

func Parse(raw []byte) (Tuning, error) {
	if len(raw) < words*4 {
		return Tuning{}, fmt.Errorf("hkeq: %d bytes, want at least %d", len(raw), words*4)
	}
	word := func(i int) int32 { return int32(binary.LittleEndian.Uint32(raw[i*4:])) }
	q29 := func(i int) float64 { return float64(word(i)) / (1 << 29) }
	q23 := func(i int) float64 { return float64(word(i)) / (1 << 23) }
	section := func(i int) dsp.Coefficients {
		return dsp.Coefficients{
			B0: q29(i),
			B1: q29(i + 1),
			B2: q29(i + 2),
			A1: -q29(i + 3),
			A2: -q29(i + 4),
		}
	}
	sections := func(from, count int) []dsp.Coefficients {
		out := make([]dsp.Coefficients, count)
		for k := range out {
			out[k] = section(from + k*5)
		}
		return out
	}
	band := func(from int) Band {
		return Band{
			Scale:     q23(from),
			Threshold: q23(from + 1),
			Floor:     q23(from + 2),
			Hold:      int(word(from + 4)),
			Release:   q23(from + 5),
			Slope:     float64(word(from + 6)),
		}
	}
	t := Tuning{
		PreDB:     float64(word(preWord)) / 10,
		PostDB:    float64(word(postWord)) / 10,
		EQ:        sections(eqWord, eqSections),
		Tail:      sections(tailWord, tailSections),
		HighPass:  section(highPassWord),
		LowPass:   section(lowPassWord),
		Low:       band(lowWord),
		High:      band(highWord),
		Lookahead: int(word(lookWord)),
	}
	if t.Lookahead <= 0 || t.Lookahead > Rate {
		return Tuning{}, fmt.Errorf("hkeq: lookahead of %d samples", t.Lookahead)
	}
	return t, nil
}

type limiter struct {
	band         Band
	gain         float64
	held         int
	delay        []float64
	at           int
	last, s1, s2 float64
}

func newLimiter(
	b Band,
	look int,
) *limiter {
	return &limiter{band: b, delay: make([]float64, look)}
}

func (l *limiter) step(x float64) float64 {
	b := l.band
	level := math.Abs(x) * l.gain * b.Scale * 32
	switch {
	case level > b.Threshold:
		l.gain *= max(1-(level-b.Threshold), b.Floor)
		l.held = b.Hold
	case l.held == 0:
		rest := 1 - l.gain
		l.gain = 1 - rest*(b.Release+(1-b.Release)*level*b.Slope/64)
	default:
		l.held--
	}
	delayed := l.delay[l.at]
	l.delay[l.at] = x
	l.at = (l.at + 1) % len(l.delay)
	smooth := bessel.B1*l.last - bessel.A1*l.s1 - bessel.A2*l.s2
	l.last, l.s2, l.s1 = l.gain, l.s1, smooth
	return delayed * smooth
}

func (l *limiter) reset() {
	clear(l.delay)
	l.gain, l.held, l.at = 0, 0, 0
	l.last, l.s1, l.s2 = 0, 0, 0
}

type Chain struct {
	pre, post   float64
	eq, tail    dsp.Cascade
	low, high   dsp.Cascade
	lowL, highL *limiter
}

func (t Tuning) Chain() *Chain {
	return &Chain{
		pre:   math.Pow(10, t.PreDB/20),
		post:  math.Pow(10, t.PostDB/20),
		eq:    dsp.NewCascade(t.EQ...),
		tail:  dsp.NewCascade(t.Tail...),
		low:   dsp.NewCascade(t.LowPass, t.LowPass),
		high:  dsp.NewCascade(t.HighPass, t.HighPass),
		lowL:  newLimiter(t.Low, t.Lookahead),
		highL: newLimiter(t.High, t.Lookahead),
	}
}

func (c *Chain) Step(x float64) float64 {
	x = c.eq.Step(x * c.pre)
	y := c.lowL.step(c.low.Step(x)) + c.highL.step(c.high.Step(x))
	return c.tail.Step(y) * c.post
}

func (c *Chain) Process(x []float32) {
	for i, v := range x {
		x[i] = float32(c.Step(float64(v)))
	}
}

func (c *Chain) Reset() {
	c.eq.Reset()
	c.tail.Reset()
	c.low.Reset()
	c.high.Reset()
	c.lowL.reset()
	c.highL.reset()
}
