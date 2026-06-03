package bunny

import (
	"sigs.k8s.io/external-dns/endpoint"
)

func recordToEndpoint(domain string, record *Record) *endpoint.Endpoint {
	ep := endpoint.NewEndpointWithTTL(
		record.Name+"."+domain,
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
