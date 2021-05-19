package types

import (
	"github.com/containernetworking/cni/pkg/types"
)

// VfState represents the state of the VF
type VfState struct {
	HostIFName   string
	AdminMAC     string
	EffectiveMAC string
	Vlan         int
}

// NetConf extends types.NetConf for sriov-cni
type NetConf struct {
	types.NetConf
	Debug         bool    `json:"debug,omitempty"` // enables debug log level
	OrigVfState   VfState // Stores the original VF state as it was prior to any operations done during cmdAdd flow
	Master        string
	MAC           string
	Representor   string // VF's representor attached to the bridge; used during deletion
	Bridge        string `json:"bridge,omitempty"` // bridge used to attach representor to it, default is "cni0"
	Vlan          int    `json:"vlan"`
	DeviceID      string `json:"deviceID"` // PCI address of a VF in valid sysfs format
	VFID          int
	ContIFNames   string // VF names after in the container; used during deletion
	RuntimeConfig struct {
		Mac string `json:"mac,omitempty"`
	} `json:"runtimeConfig,omitempty"`
}
