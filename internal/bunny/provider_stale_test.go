package bunny

import (
	"context"
	"errors"
	"testing"
	"time"
)

// flakyClient serves one zone until failing is set, then errors every
// ListZones — or, with hang, blocks until the request context is done.
type flakyClient struct {
	recordingClient
	failing bool
	hang    bool
}

func (c *flakyClient) ListZones(ctx context.Context, r ListZonesRequest) (*ListZonesResponse, error) {
	if c.hang {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if c.failing {
		return nil, errors.New("unexpected status code: 504")
	}
	return c.recordingClient.ListZones(ctx, r)
}

func newFlakyProvider(opts Options) (*flakyClient, *Provider) {
	client := &flakyClient{recordingClient: recordingClient{zone: &Zone{
		ID:      7,
		Domain:  "connect.example.com",
		Records: []*Record{{ID: 100, Name: "uat", Type: RecordTypeA, Value: "1.1.1.1"}},
	}}}
	opts.APIKey = "x"
	return client, NewProvider(client, opts)
}

// While Bunny.net is down, Records must keep serving the last successful
// listing — erroring makes external-dns exit on its first sync, which
// crash-looped every tenant's external-dns during a Bunny.net outage.
func TestRecordsServesCachedZonesWhileAPIFails(t *testing.T) {
	client, p := newFlakyProvider(Options{StaleZonesMaxAge: time.Hour})
	client.failing = true

	eps, err := p.Records(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(eps) != 1 || eps[0].DNSName != "uat.connect.example.com" {
		t.Errorf("Records = %v, want the cached uat.connect.example.com record", eps)
	}
}

// A slow API must be cut off at ZonesTimeout and fall back to the cache, so
// the webhook answers before external-dns's own read timeout expires.
func TestRecordsTimesOutToCachedZones(t *testing.T) {
	client, p := newFlakyProvider(Options{ZonesTimeout: 50 * time.Millisecond, StaleZonesMaxAge: time.Hour})
	client.hang = true

	start := time.Now()
	eps, err := p.Records(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("Records took %s, want it bounded by ZonesTimeout", elapsed)
	}
	if len(eps) != 1 {
		t.Errorf("got %d endpoints, want 1 from cache", len(eps))
	}
}

// Past StaleZonesMaxAge the cache is no longer trusted and the error surfaces.
func TestRecordsErrorsWhenCacheTooOld(t *testing.T) {
	client, p := newFlakyProvider(Options{StaleZonesMaxAge: time.Hour})
	p.lastZonesAt = time.Now().Add(-2 * time.Hour)
	client.failing = true

	if _, err := p.Records(context.Background()); err == nil {
		t.Fatal("expected an error once the cache is older than StaleZonesMaxAge")
	}
}

// Deletes and updates resolve record IDs from a live listing only; acting on
// stale IDs could touch records that no longer exist or have changed.
func TestFetchIdentifiersDoesNotUseCache(t *testing.T) {
	client, p := newFlakyProvider(Options{StaleZonesMaxAge: time.Hour})
	client.failing = true

	if _, err := p.fetchIdentifiers(context.Background(), nil); err == nil {
		t.Fatal("expected fetchIdentifiers to fail while the API is down")
	}
}
