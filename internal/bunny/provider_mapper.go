package bunny

import (
	"sigs.k8s.io/external-dns/endpoint"
)

func recordToEndpoint(domain string, record *Record) *endpoint.Endpoint {
	// An empty record name is Bunny's representation of the zone apex; the
	// endpoint DNS name is then the zone itself. Prefixing it with "." would
	// produce a name external-dns never matches, making it re-create the
	// apex record on every reconcile loop.
	dnsName := domain
	if record.Name != "" {
		dnsName = record.Name + "." + domain
	}

	ep := endpoint.NewEndpointWithTTL(
		dnsName,
		record.Type.String(),
		endpoint.TTL(record.TTLSeconds),
		record.Value,
	)

	// Bunny has no native SetIdentifier, so we persist external-dns'
	// SetIdentifier in the record Comment. Surfacing it here keeps records with
	// the same name+type but different identifiers separate (one per cluster),
	// which is what lets two clusters independently publish a health-monitored
	// A record for the same hostname without a central writer.
	ep.SetIdentifier = record.Comment

	ps := providerSpecificOptionsFromRecord(record)
	ps.ApplyToEndpoint(ep)

	return ep
}
