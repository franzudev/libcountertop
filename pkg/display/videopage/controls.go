package videopage

import (
	"time"

	"github.com/franzudev/libcountertop/pkg/display/theme"
	"github.com/franzudev/libcountertop/pkg/media/nowplaying"
)

type Controls interface {
	nowplaying.Source
	Seek(to time.Duration)
	CanSeek() bool
}

type Mark struct {
	From, To time.Duration
	Color    theme.Color
}

type Marked interface {
	Marks() []Mark
}

const linger = 4 * time.Second
