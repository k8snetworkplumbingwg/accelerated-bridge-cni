package manager

import (
	"net"

	"github.com/vishvananda/netlink"
)

// Netlink represents limited subset of functions from netlink package
type Netlink interface {
	LinkByName(string) (netlink.Link, error)
	LinkSetVfHardwareAddr(netlink.Link, int, net.HardwareAddr) error
	LinkSetHardwareAddr(netlink.Link, net.HardwareAddr) error
	LinkSetUp(netlink.Link) error
	LinkSetDown(netlink.Link) error
	LinkSetNsFd(netlink.Link, int) error
	LinkSetName(netlink.Link, string) error
	LinkSetMaster(netlink.Link, netlink.Link) error
	LinkSetNoMaster(netlink.Link) error
	BridgeVlanAdd(netlink.Link, uint16, bool, bool, bool, bool) error
	BridgeVlanDel(netlink.Link, uint16, bool, bool, bool, bool) error
}

// netlinkWrapper wrapper for netlink package
type netlinkWrapper struct {
}

// LinkByName is a wrapper for netlink.LinkByName
func (n *netlinkWrapper) LinkByName(name string) (netlink.Link, error) {
	return netlink.LinkByName(name)
}

// LinkSetVfHardwareAddr is a wrapper for netlink.LinkSetVfHardwareAddr
func (n *netlinkWrapper) LinkSetVfHardwareAddr(link netlink.Link, vf int, hwaddr net.HardwareAddr) error {
	return netlink.LinkSetVfHardwareAddr(link, vf, hwaddr)
}

// LinkSetHardwareAddr is a wrapper for netlink.LinkSetHardwareAddr
func (n *netlinkWrapper) LinkSetHardwareAddr(link netlink.Link, hwaddr net.HardwareAddr) error {
	return netlink.LinkSetHardwareAddr(link, hwaddr)
}

// LinkSetUp is a wrapper for netlink.LinkSetUp
func (n *netlinkWrapper) LinkSetUp(link netlink.Link) error {
	return netlink.LinkSetUp(link)
}

// LinkSetDown is a wrapper for netlink.LinkSetDown
func (n *netlinkWrapper) LinkSetDown(link netlink.Link) error {
	return netlink.LinkSetDown(link)
}

// LinkSetNsFd is a wrapper for netlink.LinkSetNsFd
func (n *netlinkWrapper) LinkSetNsFd(link netlink.Link, fd int) error {
	return netlink.LinkSetNsFd(link, fd)
}

// LinkSetName is a wrapper for netlink.LinkSetName
func (n *netlinkWrapper) LinkSetName(link netlink.Link, name string) error {
	return netlink.LinkSetName(link, name)
}

// LinkSetMaster is a wrapper for netlink.LinkSetMaster
func (n *netlinkWrapper) LinkSetMaster(link, master netlink.Link) error {
	return netlink.LinkSetMaster(link, master)
}

// LinkSetNoMaster is a wrapper for netlink.LinkSetNoMaster
func (n *netlinkWrapper) LinkSetNoMaster(link netlink.Link) error {
	return netlink.LinkSetNoMaster(link)
}

// BridgeVlanAdd is a wrapper for netlink.BridgeVlanAdd
func (n *netlinkWrapper) BridgeVlanAdd(link netlink.Link, vid uint16, pvid, untagged, self, master bool) error {
	return netlink.BridgeVlanAdd(link, vid, pvid, untagged, self, master)
}

// BridgeVlanDel is a wrapper for netlink.BridgeVlanDel
func (n *netlinkWrapper) BridgeVlanDel(link netlink.Link, vid uint16, pvid, untagged, self, master bool) error {
	return netlink.BridgeVlanDel(link, vid, pvid, untagged, self, master)
}

// bridgePVIDVlanAdd configure port VLAN id for link
func bridgePVIDVlanAdd(nlink Netlink, link netlink.Link, vlanID int) error {
	// pvid, egress untagged
	return nlink.BridgeVlanAdd(link, uint16(vlanID), true, true, false, true)
}

// bridgePVIDVlanDel remove port VLAN id for link
func bridgePVIDVlanDel(nlink Netlink, link netlink.Link, vlanID int) error {
	// pvid, egress untagged
	return nlink.BridgeVlanDel(link, uint16(vlanID), true, true, false, true)
}

// bridgeTrunkVlanAdd configure vlan trunk on link
func bridgeTrunkVlanAdd(nlink Netlink, link netlink.Link, vlans []int) error {
	// egress tagged
	for _, vlanID := range vlans {
		if err := nlink.BridgeVlanAdd(link, uint16(vlanID), false, false, false, true); err != nil {
			return err
		}
	}
	return nil
}
