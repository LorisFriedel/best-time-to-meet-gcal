package holidays

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestLookupRegionPrefersEmbeddedMap(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatalf("unexpected HTTP request to %s", req.URL)
			return nil, errors.New("unexpected HTTP call")
		}),
	}

	svc := NewService(client, nil)

	loc, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	code, err := svc.lookupRegion(context.Background(), "julien@example.com", loc)
	if err != nil {
		t.Fatalf("lookup region: %v", err)
	}

	if code != "FR" {
		t.Fatalf("expected FR, got %q", code)
	}
}

func TestLookupRegionFallsBackToHTTP(t *testing.T) {
	var called int
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			called++
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	svc := NewService(client, nil)

	loc := time.FixedZone("Custom/Zone", 0)

	code, err := svc.lookupRegion(context.Background(), "julien@example.com", loc)
	if err != nil {
		t.Fatalf("lookup region: %v", err)
	}

	if called != 1 {
		t.Fatalf("expected HTTP fallback to be invoked once, got %d calls", called)
	}

	if code != "" {
		t.Fatalf("expected empty code when fallback did not find a match, got %q", code)
	}
}
