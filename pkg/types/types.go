package types

import (
	"github.com/containernetworking/cni/pkg/types"
)

// VfState represents the state of the VF
type VfState struct {
	HostIFName   string `json:"host_if_name"`
	AdminMAC     string `json:"admin_mac"`
	EffectiveMAC string `json:"effective_mac"`
}

// Trunk represents configuration options for VLAN trunk
type Trunk struct {
	MinID *int `json:"minID,omitempty"`
	MaxID *int `json:"maxID,omitempty"`
	ID    *int `json:"id,omitempty"`
}

// NetConf extends types.NetConf for accelerated-bridge-cni
// defines accelerated-bridge-cni public API
type NetConf struct {
	types.NetConf
	// enable debug logging
	Debug bool `json:"debug,omitempty"`
	// bridge used to attach representor to it, default is "cni0"
	Bridge string `json:"bridge,omitempty"`
	// VLAN ID for VF
	Vlan int `json:"vlan,omitempty"`
	// VLAN Trunk configuration
	Trunk []Trunk `json:"trunk"`
	// MAC as top level config option; required for CNIs that don't support runtimeConfig
	MAC string `json:"mac,omitempty"`
	// PCI address of a VF in valid sysfs format
	DeviceID      string `json:"deviceID"`
	RuntimeConfig struct {
		Mac string `json:"mac,omitempty"`
	} `json:"runtimeConfig,omitempty"`
}

// PluginConf is a internal representation of config options and state
type PluginConf struct {
	NetConf
	// IsUserspaceDriver indicate that VF using userspace driver
	IsUserspaceDriver bool
	// Stores the original VF state as it was prior to any operations done during cmdAdd flow
	OrigVfState VfState `json:"orig_vf_state"`
	// Name of the PF to which VF belongs
	PFName string `json:"pf_name"`
	// MAC which should be set for VF
	MAC string `json:"mac"`
	// VF's representor attached to the bridge; used during deletion
	Representor string `json:"representor"`
	// VF index
	VFID int `json:"vfid"`
	// VF names after in the container; used during deletion
	ContIFNames string `json:"cont_if_names"`
	// Internal presentation of VLAN Trunk config; we don't need this option in cache
	Trunk []int `json:"-"`
}
