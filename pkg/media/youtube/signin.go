package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	cobalt      = "Mozilla/5.0 (ChromiumStylePlatform) Cobalt/Version"
	scope       = "http://gdata.youtube.com https://www.googleapis.com/auth/youtube-paid-content"
	deviceModel = "ytlr::"
	deviceGrant = "http://oauth.net/grant_type/device/1.0"
	slowDown    = 5 * time.Second
	codeEvery   = 5 * time.Second
)

var base = "https://www.youtube.com"

var (
	baseJS   = regexp.MustCompile(`<script\s+id="base-js"\s+src="([^"]+)"`)
	identity = regexp.MustCompile(`clientId:"(?P<id>[^"]+)",[^"]*?:"(?P<secret>[^"]+)"`)
)

var (
	ErrDenied  = errors.New("youtube: sign-in was refused")
	ErrExpired = errors.New("youtube: the sign-in code expired")

	errPending = errors.New("youtube: sign-in pending")
	errSlow    = errors.New("youtube: polling too fast")
)

type Client struct {
	ID     string `json:"id"`
	Secret string `json:"secret"`
}

type Code struct {
	User     string
	URL      string
	Expires  time.Time
	Interval time.Duration

	device string
}

type Token struct {
	Access  string
	Refresh string
	Expiry  time.Time
}

func TVClient(ctx context.Context, hc *http.Client) (Client, error) {
	page, err := get(ctx, hc, base+"/tv")
	if err != nil {
		return Client{}, fmt.Errorf("youtube: the TV page: %w", err)
	}
	m := baseJS.FindSubmatch(page)
	if m == nil {
		return Client{}, errors.New("youtube: the TV page has no base-js script")
	}
	src := string(m[1])
	if !strings.HasPrefix(src, "http") {
		src = base + src
	}
	script, err := get(ctx, hc, src)
	if err != nil {
		return Client{}, fmt.Errorf("youtube: the TV script: %w", err)
	}
	m = identity.FindSubmatch(script)
	if m == nil {
		return Client{}, errors.New("youtube: the TV script has no client identity")
	}
	return Client{ID: string(m[1]), Secret: string(m[2])}, nil
}

func Ask(ctx context.Context, hc *http.Client, c Client, device string) (Code, error) {
	var got struct {
		Device   string `json:"device_code"`
		User     string `json:"user_code"`
		URL      string `json:"verification_url"`
		Expires  int    `json:"expires_in"`
		Interval int    `json:"interval"`
		Error    string `json:"error"`
	}
	err := postJSON(ctx, hc, base+"/o/oauth2/device/code", map[string]string{
		"client_id":    c.ID,
		"scope":        scope,
		"device_id":    device,
		"device_model": deviceModel,
	}, &got)
	if err != nil {
		return Code{}, fmt.Errorf("youtube: asking for a sign-in code: %w", err)
	}
	if got.Error != "" || got.User == "" {
		return Code{}, fmt.Errorf("youtube: asking for a sign-in code: %q", got.Error)
	}
	every := time.Duration(got.Interval) * time.Second
	if every <= 0 {
		every = codeEvery
	}
	return Code{
		User:     got.User,
		URL:      got.URL,
		Expires:  time.Now().Add(time.Duration(got.Expires) * time.Second),
		Interval: every,
		device:   got.Device,
	}, nil
}

func Await(ctx context.Context, hc *http.Client, c Client, code Code) (Token, error) {
	ctx, cancel := context.WithDeadline(ctx, code.Expires)
	defer cancel()
	every := code.Interval
	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return Token{}, ErrExpired
			}
			return Token{}, ctx.Err()
		case <-time.After(every):
		}
		t, err := exchange(ctx, hc, map[string]string{
			"client_id":     c.ID,
			"client_secret": c.Secret,
			"code":          code.device,
			"grant_type":    deviceGrant,
		})
		switch {
		case errors.Is(err, errPending):
			continue
		case errors.Is(err, errSlow):
			every += slowDown
			continue
		case err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded):
			return Token{}, ErrExpired
		}
		return t, err
	}
}

func Refresh(ctx context.Context, hc *http.Client, c Client, refresh string) (Token, error) {
	t, err := exchange(ctx, hc, map[string]string{
		"client_id":     c.ID,
		"client_secret": c.Secret,
		"refresh_token": refresh,
		"grant_type":    "refresh_token",
	})
	if err != nil {
		return Token{}, err
	}
	if t.Refresh == "" {
		t.Refresh = refresh
	}
	return t, nil
}

func exchange(ctx context.Context, hc *http.Client, body map[string]string) (Token, error) {
	var got struct {
		Access  string `json:"access_token"`
		Refresh string `json:"refresh_token"`
		Expires int    `json:"expires_in"`
		Error   string `json:"error"`
	}
	if err := postJSON(ctx, hc, base+"/o/oauth2/token", body, &got); err != nil {
		return Token{}, fmt.Errorf("youtube: token: %w", err)
	}
	switch got.Error {
	case "":
	case "authorization_pending":
		return Token{}, errPending
	case "slow_down":
		return Token{}, errSlow
	case "access_denied":
		return Token{}, ErrDenied
	case "expired_token":
		return Token{}, ErrExpired
	default:
		return Token{}, fmt.Errorf("youtube: token: %q", got.Error)
	}
	if got.Access == "" {
		return Token{}, errors.New("youtube: token: no access token")
	}
	return Token{
		Access:  got.Access,
		Refresh: got.Refresh,
		Expiry:  time.Now().Add(time.Duration(got.Expires) * time.Second),
	}, nil
}

func get(ctx context.Context, hc *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", cobalt)
	req.Header.Set("Referer", base+"/tv")
	status, body, err := send(hc, req)
	if err != nil {
		return nil, err
	}
	if status/100 != 2 {
		return nil, fmt.Errorf("%s: %d", req.URL.Path, status)
	}
	return body, nil
}

func postJSON(ctx context.Context, hc *http.Client, url string, body, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	status, got, err := send(hc, req)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(got, out); err != nil {
		return fmt.Errorf("%s: %d: %w", req.URL.Path, status, err)
	}
	return nil
}

func send(hc *http.Client, req *http.Request) (int, []byte, error) {
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return resp.StatusCode, body, err
}
