package plugin

import "github.com/containernetworking/plugins/pkg/ns"

// NS represents limited subset of functions from ns package
type NS interface {
	GetNS(nspath string) (ns.NetNS, error)
}

// nsWrapper wrapper for ns package
type nsWrapper struct{}

// GetNS is a wrapper for ns.GetNS
func (nsw *nsWrapper) GetNS(nspath string) (ns.NetNS, error) {
	return ns.GetNS(nspath)
}
