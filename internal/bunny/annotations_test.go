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
