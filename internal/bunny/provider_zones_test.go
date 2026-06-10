package bunny

import (
	"context"
	"testing"

	"sigs.k8s.io/external-dns/endpoint"
)

// extractRecordComponents must match zones on label boundaries, support the
// zone apex (empty record name — previously a slice-bounds panic) and prefer
// the most specific zone when zones nest.
func TestExtractRecordComponents(t *testing.T) {
	zones := []string{"example.com", "connect.example.com", "other.net"}

	cases := []struct {
		name       string
		dnsName    string
		wantRecord string
		wantZone   string
		wantOK     bool
	}{
		{"subdomain", "app.example.com", "app", "example.com", true},
		{"multi-label record", "a.b.example.com", "a.b", "example.com", true},
		{"zone apex", "example.com", "", "example.com", true},
		{"nested zone apex", "connect.example.com", "", "connect.example.com", true},
		{"nested zone wins over parent", "uat.connect.example.com", "uat", "connect.example.com", true},
		{"no matching zone", "app.unrelated.org", "", "", false},
		{"suffix without label boundary", "foo-example.com", "", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record, zone, ok := extractRecordComponents(zones, tc.dnsName)
			if ok != tc.wantOK || record != tc.wantRecord || zone != tc.wantZone {
				t.Errorf("extractRecordComponents(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tc.dnsName, record, zone, ok, tc.wantRecord, tc.wantZone, tc.wantOK)
			}
		})
	}
}

// recordingClient serves one zone and records every CreateRecord call.
type recordingClient struct {
	zone    *Zone
	created []CreateRecordRequest
}

func (c *recordingClient) ListZones(_ context.Context, _ ListZonesRequest) (*ListZonesResponse, error) {
	return &ListZonesResponse{Items: []*Zone{c.zone}, HasMoreItems: false}, nil
}

func (c *recordingClient) CreateRecord(_ context.Context, _ string, req CreateRecordRequest) (*Record, error) {
	c.created = append(c.created, req)
	return &Record{ID: int64(len(c.created)), Name: req.Name, Type: req.Type, Value: req.Value}, nil
}

func (c *recordingClient) UpdateRecord(_ context.Context, _ int64, _ int64, _ UpdateRecordRequest) error {
	return nil
}
func (c *recordingClient) DeleteRecord(_ context.Context, _ int64, _ int64) error { return nil }

// Creates for hostnames outside every owned zone (e.g. the platform's own
// ingress hosts) must be skipped, not fail the batch — a failing batch wedges
// external-dns in a crash loop and blocks the valid records that follow.
func TestCreateEndpointsSkipsRecordsOutsideOwnedZones(t *testing.T) {
	client := &recordingClient{zone: &Zone{ID: 7, Domain: "connect.example.com"}}
	p := NewProvider(client, Options{APIKey: "x"})

	creates := []*endpoint.Endpoint{
		endpoint.NewEndpoint("abc123.apps.platform.cloud", "A", "9.9.9.9"),
		endpoint.NewEndpoint("uat.connect.example.com", "A", "1.1.1.1"),
	}

	if err := p.createEndpoints(context.Background(), creates); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(client.created) != 1 {
		t.Fatalf("created %d records, want 1: %+v", len(client.created), client.created)
	}
	if client.created[0].Name != "uat" {
		t.Errorf("created record name = %q, want uat", client.created[0].Name)
	}
}

// An apex record (dnsName == zone) must be created with an empty record name —
// this is how Bunny represents the apex. This used to panic with
// "slice bounds out of range [:-1]".
func TestCreateEndpointsApexRecord(t *testing.T) {
	client := &recordingClient{zone: &Zone{ID: 7, Domain: "connect.example.com"}}
	p := NewProvider(client, Options{APIKey: "x"})

	creates := []*endpoint.Endpoint{
		endpoint.NewEndpoint("connect.example.com", "A", "1.1.1.1"),
	}

	if err := p.createEndpoints(context.Background(), creates); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(client.created) != 1 {
		t.Fatalf("created %d records, want 1", len(client.created))
	}
	if client.created[0].Name != "" {
		t.Errorf("apex record name = %q, want empty", client.created[0].Name)
	}
}

// An apex record (empty Bunny record name) must read back as the zone itself,
// not ".zone" — otherwise external-dns never recognizes the record it created
// and re-creates it on every reconcile loop, flooding the zone.
func TestRecordToEndpointApexRoundTrip(t *testing.T) {
	ep := recordToEndpoint("connect.example.com", &Record{Name: "", Type: RecordTypeA, Value: "1.1.1.1"})
	if ep.DNSName != "connect.example.com" {
		t.Errorf("DNSName = %q, want connect.example.com", ep.DNSName)
	}
}

// fetchIdentifiers must leave out-of-zone endpoints out of the map (so
// delete/update skip them) instead of erroring the whole lookup.
func TestFetchIdentifiersSkipsRecordsOutsideOwnedZones(t *testing.T) {
	client := &recordingClient{zone: &Zone{
		ID:     7,
		Domain: "connect.example.com",
		Records: []*Record{
			{ID: 100, Name: "uat", Type: RecordTypeA, Value: "1.1.1.1"},
		},
	}}
	p := NewProvider(client, Options{APIKey: "x"})

	eps := []*endpoint.Endpoint{
		endpoint.NewEndpoint("abc123.apps.platform.cloud", "A", "9.9.9.9"),
		endpoint.NewEndpoint("uat.connect.example.com", "A", "1.1.1.1"),
	}

	tuples, err := p.fetchIdentifiers(context.Background(), eps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tuples) != 1 {
		t.Fatalf("got %d tuples, want 1: %+v", len(tuples), tuples)
	}
	if got := tuples[identifierKey("uat.connect.example.com", "A", "")]; got.RecordID != 100 {
		t.Errorf("RecordID = %d, want 100", got.RecordID)
	}
}
