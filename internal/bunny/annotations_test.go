package bunny

import (
	"testing"

	"sigs.k8s.io/external-dns/endpoint"
)

// A Bunny record's monitoring settings must survive the
// Record -> endpoint (ApplyToEndpoint) -> options (FromEndpoint) round-trip so
// the plan does not see spurious diffs and monitoring is actually applied.
func TestMonitoringRoundTrip(t *testing.T) {
	rec := &Record{
		MonitorType: MonitorTypeHTTP,
		Port:        443,
		Weight:      50,
		Disabled:    true,
	}

	ep := endpoint.NewEndpoint("foo.example.com", "A", "1.2.3.4")
	providerSpecificOptionsFromRecord(rec).ApplyToEndpoint(ep)

	opts, err := providerSpecificOptionsFromEndpoint(ep)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if opts.MonitorType != MonitorTypeHTTP {
		t.Errorf("MonitorType = %v, want http", opts.MonitorType)
	}
	if opts.Port != 443 {
		t.Errorf("Port = %d, want 443", opts.Port)
	}
	if opts.Weight != 50 {
		t.Errorf("Weight = %d, want 50", opts.Weight)
	}
	if !opts.Disabled {
		t.Errorf("Disabled = false, want true")
	}
}

func TestMonitoringDefaults(t *testing.T) {
	ep := endpoint.NewEndpoint("foo.example.com", "A", "1.2.3.4")

	opts, err := providerSpecificOptionsFromEndpoint(ep)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if opts.MonitorType != MonitorTypeNone {
		t.Errorf("MonitorType = %v, want none", opts.MonitorType)
	}
	if opts.Port != 0 {
		t.Errorf("Port = %d, want 0", opts.Port)
	}
	if opts.Weight != 100 {
		t.Errorf("Weight = %d, want 100 (default)", opts.Weight)
	}
}

// TCP monitoring (Bunny MonitorType 3) is the right monitor for non-HTTP
// services like databases. It must parse from the annotation and survive the
// round-trip with its port.
func TestTCPMonitorRoundTrip(t *testing.T) {
	if got := MonitorTypeFromString("tcp"); got != MonitorTypeTCP {
		t.Fatalf("MonitorTypeFromString(tcp) = %v, want tcp", got)
	}
	if got := MonitorTypeTCP.String(); got != "tcp" {
		t.Fatalf("MonitorTypeTCP.String() = %q, want tcp", got)
	}
	if int(MonitorTypeTCP) != 3 {
		t.Fatalf("MonitorTypeTCP = %d, want 3 (Bunny code)", int(MonitorTypeTCP))
	}

	rec := &Record{MonitorType: MonitorTypeTCP, Port: 5432}
	ep := endpoint.NewEndpoint("db.example.com", "A", "1.2.3.4")
	providerSpecificOptionsFromRecord(rec).ApplyToEndpoint(ep)

	opts, err := providerSpecificOptionsFromEndpoint(ep)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.MonitorType != MonitorTypeTCP {
		t.Errorf("MonitorType = %v, want tcp", opts.MonitorType)
	}
	if opts.Port != 5432 {
		t.Errorf("Port = %d, want 5432", opts.Port)
	}
}
