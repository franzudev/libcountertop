package covers

import (
	"container/list"
	"context"
	"errors"
	"image"
	"log/slog"
	"strconv"
	"sync"
	"time"

	gogui "github.com/go-gui-org/go-gui/gui"
	xdraw "golang.org/x/image/draw"

	"github.com/ygelfand/libcountertop/pkg/display/picture"
	"github.com/ygelfand/libcountertop/pkg/runtime/safe"
)

const (
	keep     = 96
	parallel = 4
	retry    = time.Minute
	askTime  = 15 * time.Second
)

type Shape int

const (
	Square Shape = iota
	Wide
)

type Options struct {
	Prefix string
	Shape  Shape
	Get    func(ctx context.Context, url string) ([]byte, error)
	Redraw func()
	Keep   int
}

type Covers struct {
	o Options

	mu      sync.Mutex
	byURL   map[string]*cover
	order   *list.List
	loading int
}

type cover struct {
	url    string
	raw    []byte
	src    string
	key    string
	side   int
	busy   bool
	failed time.Time
	place  *list.Element
}

func New(o Options) *Covers {
	if o.Keep <= 0 {
		o.Keep = keep
	}
	return &Covers{o: o, byURL: map[string]*cover{}, order: list.New()}
}

func (c *Covers) Src(url string, side int) string {
	if url == "" || side <= 0 {
		return ""
	}
	c.mu.Lock()
	k := c.byURL[url]
	if k == nil {
		k = &cover{url: url}
		k.place = c.order.PushBack(k)
		c.byURL[url] = k
	} else {
		c.order.MoveToBack(k.place)
	}
	c.trim()
	if k.src != "" && !gogui.HasImage(k.key) {
		k.src, k.side = "", 0
	}
	due := !k.busy && c.loading < parallel &&
		(k.raw == nil && time.Since(k.failed) >= retry || k.raw != nil && side > k.side)
	if due {
		k.busy = true
		c.loading++
	}
	src, raw := k.src, k.raw
	c.mu.Unlock()
	if due {
		safe.Go("cover", func() { c.load(url, k, raw, side) })
	}
	return src
}

func (c *Covers) trim() {
	for c.order.Len() > c.o.Keep {
		oldest := c.order.Front()
		gone := oldest.Value.(*cover)
		c.order.Remove(oldest)
		delete(c.byURL, gone.url)
		if gone.key != "" {
			gogui.DropImage(gone.key)
		}
	}
}

func (c *Covers) load(url string, k *cover, raw []byte, side int) {
	var err error
	if raw == nil {
		slog.Debug("cover download", "prefix", c.o.Prefix, "url", url)
		ctx, cancel := context.WithTimeout(context.Background(), askTime)
		raw, err = c.o.Get(ctx, url)
		cancel()
	}
	src, key := "", c.key(url, side)
	if err == nil {
		src, err = c.register(key, raw, side)
	}
	c.mu.Lock()
	k.busy = false
	c.loading--
	if err == nil {
		if k.key != "" && k.key != key {
			gogui.DropImage(k.key)
		}
		k.raw, k.src, k.key, k.side = raw, src, key, side
	} else {
		k.failed = time.Now()
	}
	c.mu.Unlock()
	if err != nil {
		slog.Debug("cover", "url", url, "err", err)
		return
	}
	if c.o.Redraw != nil {
		c.o.Redraw()
	}
}

func (c *Covers) register(key string, raw []byte, side int) (string, error) {
	img, err := picture.Decode(raw, image.Point{X: side, Y: side}, picture.Smooth)
	if err != nil {
		return "", err
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return "", errors.New("covers: empty picture")
	}
	crop, w, h := b, min(side, b.Dx()), 0
	h = w * b.Dy() / b.Dx()
	if c.o.Shape == Square {
		edge := min(b.Dx(), b.Dy())
		crop = image.Rect(0, 0, edge, edge).Add(b.Min).
			Add(image.Pt((b.Dx()-edge)/2, (b.Dy()-edge)/2))
		w = min(side, edge)
		h = w
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, max(h, 1)))
	xdraw.ApproxBiLinear.Scale(out, out.Rect, img, crop, xdraw.Src, nil)
	return gogui.UseImage(key, w, max(h, 1), out.Pix), nil
}

func (c *Covers) key(url string, side int) string {
	return c.o.Prefix + "/" + strconv.Itoa(side) + "/" + url
}
