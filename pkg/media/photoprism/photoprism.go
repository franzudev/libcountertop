// Package photoprism is an HTTP client for the public REST API of the
// PhotoPrism photo server, authenticated with an API key sent as a bearer
// token in the Authorization header.
package photoprism

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ygelfand/libcountertop/pkg/fetch"
)

const maxPicture = 64 << 20

// The client never reads more photos than this per search.
const photosMost = 500

var (
	ErrKey      = errors.New("photoprism: the API key was refused")
	ErrNoServer = errors.New("photoprism: no server set")
	ErrNotFound = errors.New("photoprism: not found")
)

type Client struct {
	Base string
	Key  string
	HTTP *http.Client
}

type Album struct {
	UID   string `json:"UID"`
	Type  string `json:"Type"`
	Title string `json:"Title"`
}

type Photo struct {
	UID          string `json:"UID"`
	Type         string `json:"Type"`
	TakenAt      string `json:"TakenAt"`
	TakenAtLocal string `json:"TakenAtLocal"`
	FileName     string `json:"FileName"`
}

func New(base, key string) *Client {
	return &Client{
		Base: strings.TrimRight(base, "/"),
		Key:  key,
		HTTP: fetch.Client(30 * time.Second),
	}
}

// Photos walks GET /api/v1/photos for one album, an empty album meaning every
// photo in the index, newest first, paging with count/offset until the server
// runs out or photosMost images have been read.
func (c *Client) Photos(ctx context.Context, album string) ([]Photo, error) {
	query := url.Values{"order": {"newest"}, "type": {"image"}}
	if album != "" {
		query.Set("album", album)
	}
	var all []Photo
	offset := 0
	for len(all) < photosMost {
		page := query
		page.Set("count", strconv.Itoa(photosMost))
		page.Set("offset", strconv.Itoa(offset))
		var out []Photo
		if err := c.call(
			ctx, http.MethodGet, "/api/v1/photos?"+page.Encode(), &out,
		); err != nil {
			return nil, err
		}
		for _, p := range out {
			if p.Type == "image" {
				all = append(all, p)
				if len(all) >= photosMost {
					break
				}
			}
		}
		offset += len(out)
		if len(out) == 0 {
			return all, nil
		}
	}
	return all, nil
}

// Albums reads GET /api/v1/albums, keeping the plain albums and dropping the
// canned ones the server may add.
func (c *Client) Albums(ctx context.Context) ([]Album, error) {
	var out []Album
	if err := c.call(ctx, http.MethodGet, "/api/v1/albums?count=1000&type=album", &out); err != nil {
		return nil, err
	}
	var all []Album
	for _, a := range out {
		if a.Type == "" || a.Type == "album" {
			all = append(all, a)
		}
	}
	return all, nil
}

// Picture downloads the original file of a photo. The server serves it as it
// indexed it; the local shrink pipeline resizes afterwards when needed.
func (c *Client) Picture(ctx context.Context, uid string) ([]byte, error) {
	path := "/api/v1/photos/" + url.PathEscape(uid) + "/dl"
	req, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "image/*")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("photoprism: %w", err)
	}
	defer resp.Body.Close()
	if err := status(resp); err != nil {
		return nil, err
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxPicture+1))
	if err != nil {
		return nil, fmt.Errorf("photoprism: picture of %s: %w", uid, err)
	}
	if len(b) > maxPicture {
		return nil, fmt.Errorf("photoprism: picture of %s is over %d bytes", uid, maxPicture)
	}
	return b, nil
}

func (c *Client) call(ctx context.Context, method, path string, out any) error {
	req, err := c.request(ctx, method, path, nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("photoprism: %w", err)
	}
	defer resp.Body.Close()
	if err := status(resp); err != nil {
		return err
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(out); err != nil {
		return fmt.Errorf("photoprism: %s %s: %w", method, path, err)
	}
	return nil
}

func status(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrKey
	case http.StatusNotFound:
		return ErrNotFound
	}
	msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf(
		"photoprism: %s: %s %s",
		resp.Request.URL,
		resp.Status,
		strings.TrimSpace(string(msg)),
	)
}

func (c *Client) request(
	ctx context.Context,
	method, path string,
	body io.Reader,
) (*http.Request, error) {
	if c.Base == "" {
		return nil, ErrNoServer
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Base+path, body)
	if err != nil {
		return nil, fmt.Errorf("photoprism: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("Accept", "application/json")
	return req, nil
}
