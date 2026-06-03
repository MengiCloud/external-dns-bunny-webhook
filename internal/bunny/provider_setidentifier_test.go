package bunny

import (
	"context"
	"testing"

	"sigs.k8s.io/external-dns/endpoint"
)

// recordToEndpoint must surface the Bunny record Comment as the endpoint
// SetIdentifier so external-dns keeps same-name+type records (one per cluster)
// separate instead of merging them into a single multi-target endpoint.
func TestRecordToEndpointSetIdentifierRoundTrip(t *testing.T) {
	rec := &Record{
		Name:    "app",
		Type:    RecordTypeA,
		Value:   "1.2.3.4",
		Comment: "cluster-abc",
	}

	ep := recordToEndpoint("example.com", rec)

	if ep.DNSName != "app.example.com" {
		t.Errorf("DNSName = %q, want app.example.com", ep.DNSName)
	}
	if ep.SetIdentifier != "cluster-abc" {
		t.Errorf("SetIdentifier = %q, want cluster-abc", ep.SetIdentifier)
	}
}

// A record without a Comment must produce an empty SetIdentifier (the normal
// single-owner platform record), preserving backward-compatible behavior.
func TestRecordToEndpointNoSetIdentifier(t *testing.T) {
	ep := recordToEndpoint("example.com", &Record{Name: "api", Type: RecordTypeA, Value: "1.2.3.4"})
	if ep.SetIdentifier != "" {
		t.Errorf("SetIdentifier = %q, want empty", ep.SetIdentifier)
	}
}

// identifierKey must distinguish records that share a name+type but have
// different SetIdentifiers, otherwise two clusters' records for one hostname
// would resolve to the same Bunny record and clobber each other.
func TestIdentifierKeyDisambiguatesBySetIdentifier(t *testing.T) {
	a := identifierKey("app.example.com", "A", "cluster-a")
	b := identifierKey("app.example.com", "A", "cluster-b")
	if a == b {
		t.Fatalf("identifierKey collided across SetIdentifiers: %q", a)
	}
}

// fakeZonesClient serves a fixed zone for fetchIdentifiers/Records tests.
type fakeZonesClient struct {
	zone *Zone
}

func (f *fakeZonesClient) ListZones(_ context.Context, _ ListZonesRequest) (*ListZonesResponse, error) {
	return &ListZonesResponse{Items: []*Zone{f.zone}, HasMoreItems: false}, nil
}
func (f *fakeZonesClient) CreateRecord(_ context.Context, _ string, _ CreateRecordRequest) (*Record, error) {
	return nil, nil
}
func (f *fakeZonesClient) UpdateRecord(_ context.Context, _ int64, _ int64, _ UpdateRecordRequest) error {
	return nil
}
func (f *fakeZonesClient) DeleteRecord(_ context.Context, _ int64, _ int64) error { return nil }

// Two A records for the same hostname, distinguished only by SetIdentifier
// (Comment), must resolve to their own record IDs — this is what makes
// decentralized per-cluster failover records addressable.
func TestFetchIdentifiersResolvesPerSetIdentifier(t *testing.T) {
	zone := &Zone{
		ID:     7,
		Domain: "example.com",
		Records: []*Record{
			{ID: 100, Name: "app", Type: RecordTypeA, Value: "1.1.1.1", Comment: "cluster-a"},
			{ID: 200, Name: "app", Type: RecordTypeA, Value: "2.2.2.2", Comment: "cluster-b"},
		},
	}

	p := NewProvider(&fakeZonesClient{zone: zone}, Options{APIKey: "x"})

	epA := endpoint.NewEndpoint("app.example.com", "A", "1.1.1.1")
	epA.SetIdentifier = "cluster-a"
	epB := endpoint.NewEndpoint("app.example.com", "A", "2.2.2.2")
	epB.SetIdentifier = "cluster-b"

	tuples, err := p.fetchIdentifiers(context.Background(), []*endpoint.Endpoint{epA, epB})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotA := tuples[identifierKey("app.example.com", "A", "cluster-a")]
	gotB := tuples[identifierKey("app.example.com", "A", "cluster-b")]

	if gotA.RecordID != 100 {
		t.Errorf("cluster-a RecordID = %d, want 100", gotA.RecordID)
	}
	if gotB.RecordID != 200 {
		t.Errorf("cluster-b RecordID = %d, want 200", gotB.RecordID)
	}
}
