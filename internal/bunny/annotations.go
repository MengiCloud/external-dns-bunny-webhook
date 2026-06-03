package bunny

import (
	"strconv"

	"sigs.k8s.io/external-dns/endpoint"
)

const (
	providerSpecificDisabled    = "webhook/bunny-disabled"
	providerSpecificMonitorType = "webhook/bunny-monitor-type"
	providerSpecificPort        = "webhook/bunny-port"
	providerSpecificWeight      = "webhook/bunny-weight"
)

type providerSpecificOptions struct {
	Disabled    bool
	MonitorType MonitorType
	// Port is the TCP port the health monitor checks (used with ping/http
	// MonitorType). 0 lets Bunny use the protocol default.
	Port   int
	Weight int
}

func providerSpecificOptionsFromEndpoint(e *endpoint.Endpoint) (providerSpecificOptions, error) {
	opts := providerSpecificOptions{}

	if disabled, ok := e.GetProviderSpecificProperty(providerSpecificDisabled); ok {
		var err error
		opts.Disabled, err = strconv.ParseBool(disabled)
		if err != nil {
			opts.Disabled = false
		}
	}

	if monitorType, ok := e.GetProviderSpecificProperty(providerSpecificMonitorType); ok {
		opts.MonitorType = MonitorTypeFromString(monitorType)
	}

	if port, ok := e.GetProviderSpecificProperty(providerSpecificPort); ok {
		if v, err := strconv.Atoi(port); err == nil && v >= 0 && v <= 65535 {
			opts.Port = v
		}
	}

	if weight, ok := e.GetProviderSpecificProperty(providerSpecificWeight); ok {
		var err error
		opts.Weight, err = strconv.Atoi(weight)
		if err != nil {
			opts.Weight = 100
		}

		if opts.Weight < 1 {
			opts.Weight = 1
		}

		if opts.Weight > 100 {
			opts.Weight = 100
		}
	}

	if opts.Weight == 0 {
		opts.Weight = 100
	}

	return opts, nil
}

func providerSpecificOptionsFromRecord(r *Record) *providerSpecificOptions {
	opts := &providerSpecificOptions{
		MonitorType: r.MonitorType,
		Port:        r.Port,
		Weight:      r.Weight,
		Disabled:    r.Disabled,
	}

	return opts
}

func (p *providerSpecificOptions) ApplyToEndpoint(e *endpoint.Endpoint) {
	e.WithProviderSpecific(providerSpecificMonitorType, p.MonitorType.String())
	e.WithProviderSpecific(providerSpecificPort, strconv.Itoa(p.Port))
	e.WithProviderSpecific(providerSpecificWeight, strconv.Itoa(p.Weight))
	e.WithProviderSpecific(providerSpecificDisabled, strconv.FormatBool(p.Disabled))
}
