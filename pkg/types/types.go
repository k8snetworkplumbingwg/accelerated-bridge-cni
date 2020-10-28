package types

import (
	"github.com/containernetworking/cni/pkg/types"
	"github.com/vishvananda/netlink"
)

// VfState represents the state of the VF
type VfState struct {
	HostIFName   string
	SpoofChk     bool
	AdminMAC     string
	EffectiveMAC string
	Vlan         int
	VlanQoS      int
	MinTxRate    int
	MaxTxRate    int
	LinkState    uint32
}

// FillFromVfInfo - Fill attributes according to the provided netlink.VfInfo struct
func (vs *VfState) FillFromVfInfo(info *netlink.VfInfo) {
	vs.AdminMAC = info.Mac.String()
	vs.LinkState = info.LinkState
	vs.MaxTxRate = int(info.MaxTxRate)
	vs.MinTxRate = int(info.MinTxRate)
	vs.Vlan = info.Vlan
	vs.VlanQoS = info.Qos
	vs.SpoofChk = info.Spoofchk
}

// VfConf is a cached configuration used during cmdDel
type VfConf struct {
	types.NetConf
	ConfVer     string `json:"config_version"`
	VFID        int
	OrigVfState VfState // Stores the original VF state as it was prior to any operations done during cmdAdd flow
	Master      string
	Bridge      string `json:"bridge,omitempty"` // bridge used to attach representor to it, default is "cni0"
	Representor string // VF's representor attached to the bridge
	ContIFNames string // VF names after in the container
	Trust       string `json:"trust,omitempty"`      // on|off
	LinkState   string `json:"link_state,omitempty"` // auto|enable|disable
}

// NetConf extends types.NetConf for accelerated-bridge-cni
type NetConf struct {
	VfConf
	DeviceID      string `json:"deviceID"` // PCI address of a VF in valid sysfs format
	Vlan          *int   `json:"vlan"`     // XXX there was no decision regarding vlan support; keep it here
	VlanQoS       *int   `json:"vlanQoS"`
	MinTxRate     *int   `json:"min_tx_rate"`        // Mbps, 0 = disable rate limiting
	MaxTxRate     *int   `json:"max_tx_rate"`        // Mbps, 0 = disable rate limiting
	SpoofChk      string `json:"spoofchk,omitempty"` // on|off
	Debug         bool   `json:"debug,omitempty"`
	RuntimeConfig struct {
		Mac string `json:"mac,omitempty"`
	} `json:"runtimeConfig,omitempty"`
}
