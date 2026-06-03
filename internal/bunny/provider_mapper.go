package bunny

import (
	"sigs.k8s.io/external-dns/endpoint"
)

func recordToEndpoint(domain string, record *Record) *endpoint.Endpoint {
	value := record.Value
	// Bunny stores TXT record values with surrounding double quotes, but
	// external-dns works with the unquoted value. Without stripping them the
	// read-back never matches the desired (unquoted) value, so external-dns
	// re-updates the registry TXT records on every reconcile. Strip a single
	// surrounding quote pair to keep the round-trip stable.
	if record.Type == RecordTypeTXT && len(value) >= 2 &&
		value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
	}

	ep := endpoint.NewEndpointWithTTL(
		record.Name+"."+domain,
		record.Type.String(),
		endpoint.TTL(record.TTLSeconds),
		value,
	)

	ps := providerSpecificOptionsFromRecord(record)
	ps.ApplyToEndpoint(ep)

	return ep
}
