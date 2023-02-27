package plugin

import (
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ipam"
)

// IPAM represents limited subset of functions from ipam package
type IPAM interface {
	ExecAdd(plugin string, netconf []byte) (types.Result, error)
	ExecDel(plugin string, netconf []byte) error
	ConfigureIface(ifName string, res *current.Result) error
}

// ipamWrapper wrapper for ipam package
type ipamWrapper struct{}

// ExecAdd is a wrapper for ipam.ExecAdd
func (i *ipamWrapper) ExecAdd(plugin string, netconf []byte) (types.Result, error) {
	return ipam.ExecAdd(plugin, netconf)
}

// ExecDel is a wrapper for ipam.ExecDel
func (i *ipamWrapper) ExecDel(plugin string, netconf []byte) error {
	return ipam.ExecDel(plugin, netconf)
}

// ConfigureIface is a wrapper for ipam.ConfigureIface
func (i *ipamWrapper) ConfigureIface(ifName string, res *current.Result) error {
	return ipam.ConfigureIface(ifName, res)
}
