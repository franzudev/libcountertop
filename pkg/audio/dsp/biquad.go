package dsp

type Coefficients struct{ B0, B1, B2, A1, A2 float64 }

type Biquad struct {
	c      Coefficients
	z1, z2 float64
}

func NewBiquad(c Coefficients) *Biquad { return &Biquad{c: c} }

func (f *Biquad) Step(x float64) float64 {
	y := f.c.B0*x + f.z1
	f.z1 = f.c.B1*x - f.c.A1*y + f.z2
	f.z2 = f.c.B2*x - f.c.A2*y
	return y
}

func (f *Biquad) Reset() { f.z1, f.z2 = 0, 0 }

type Cascade []*Biquad

func NewCascade(cs ...Coefficients) Cascade {
	out := make(Cascade, len(cs))
	for i, c := range cs {
		out[i] = NewBiquad(c)
	}
	return out
}

func (c Cascade) Step(x float64) float64 {
	for _, f := range c {
		x = f.Step(x)
	}
	return x
}

func (c Cascade) Reset() {
	for _, f := range c {
		f.Reset()
	}
}
