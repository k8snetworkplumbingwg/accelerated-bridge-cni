package manager

import (
	"fmt"
	"net"

	"github.com/Mellanox/sriovnet"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/rs/zerolog/log"
	"github.com/vishvananda/netlink"

	"github.com/DmytroLinkin/accelerated-bridge-cni/pkg/types"
	"github.com/DmytroLinkin/accelerated-bridge-cni/pkg/utils"
)

const (
	ON = "on"
)

// mocked netlink interface
// required for unit tests

// NetlinkManager is an interface to mock netlink library
type NetlinkManager interface {
	LinkByName(string) (netlink.Link, error)
	LinkSetVfVlan(netlink.Link, int, int) error
	LinkSetVfVlanQos(netlink.Link, int, int, int) error
	LinkSetVfHardwareAddr(netlink.Link, int, net.HardwareAddr) error
	LinkSetHardwareAddr(netlink.Link, net.HardwareAddr) error
	LinkSetUp(netlink.Link) error
	LinkSetDown(netlink.Link) error
	LinkSetNsFd(netlink.Link, int) error
	LinkSetName(netlink.Link, string) error
	LinkSetVfRate(netlink.Link, int, int, int) error
	LinkSetVfSpoofchk(netlink.Link, int, bool) error
	LinkSetVfTrust(netlink.Link, int, bool) error
	LinkSetVfState(netlink.Link, int, uint32) error
	LinkSetMaster(netlink.Link, netlink.Link) error
	LinkSetNoMaster(netlink.Link) error
}

// MyNetlink NetlinkManager
type MyNetlink struct {
}

// LinkByName implements NetlinkManager
func (n *MyNetlink) LinkByName(name string) (netlink.Link, error) {
	return netlink.LinkByName(name)
}

// LinkSetVfVlan using NetlinkManager
func (n *MyNetlink) LinkSetVfVlan(link netlink.Link, vf, vlan int) error {
	return netlink.LinkSetVfVlan(link, vf, vlan)
}

// LinkSetVfVlanQos sets VLAN ID and QoS field for given VF using NetlinkManager
func (n *MyNetlink) LinkSetVfVlanQos(link netlink.Link, vf, vlan, qos int) error {
	return netlink.LinkSetVfVlanQos(link, vf, vlan, qos)
}

// LinkSetVfHardwareAddr using NetlinkManager
func (n *MyNetlink) LinkSetVfHardwareAddr(link netlink.Link, vf int, hwaddr net.HardwareAddr) error {
	return netlink.LinkSetVfHardwareAddr(link, vf, hwaddr)
}

// LinkSetHardwareAddr using NetlinkManager
func (n *MyNetlink) LinkSetHardwareAddr(link netlink.Link, hwaddr net.HardwareAddr) error {
	return netlink.LinkSetHardwareAddr(link, hwaddr)
}

// LinkSetUp using NetlinkManager
func (n *MyNetlink) LinkSetUp(link netlink.Link) error {
	return netlink.LinkSetUp(link)
}

// LinkSetDown using NetlinkManager
func (n *MyNetlink) LinkSetDown(link netlink.Link) error {
	return netlink.LinkSetDown(link)
}

// LinkSetNsFd using NetlinkManager
func (n *MyNetlink) LinkSetNsFd(link netlink.Link, fd int) error {
	return netlink.LinkSetNsFd(link, fd)
}

// LinkSetName using NetlinkManager
func (n *MyNetlink) LinkSetName(link netlink.Link, name string) error {
	return netlink.LinkSetName(link, name)
}

// LinkSetVfRate using NetlinkManager
func (n *MyNetlink) LinkSetVfRate(link netlink.Link, vf, minRate, maxRate int) error {
	return netlink.LinkSetVfRate(link, vf, minRate, maxRate)
}

// LinkSetVfSpoofchk using NetlinkManager
func (n *MyNetlink) LinkSetVfSpoofchk(link netlink.Link, vf int, check bool) error {
	return netlink.LinkSetVfSpoofchk(link, vf, check)
}

// LinkSetVfTrust using NetlinkManager
func (n *MyNetlink) LinkSetVfTrust(link netlink.Link, vf int, state bool) error {
	return netlink.LinkSetVfTrust(link, vf, state)
}

// LinkSetVfState using NetlinkManager
func (n *MyNetlink) LinkSetVfState(link netlink.Link, vf int, state uint32) error {
	return netlink.LinkSetVfState(link, vf, state)
}

// LinkSetMaster using NetlinkManager
func (n *MyNetlink) LinkSetMaster(link, master netlink.Link) error {
	return netlink.LinkSetMaster(link, master)
}

// LinkSetNoMaster using NetlinkManager
func (n *MyNetlink) LinkSetNoMaster(link netlink.Link) error {
	return netlink.LinkSetNoMaster(link)
}

type pciUtils interface {
	getSriovNumVfs(ifName string) (int, error)
	getVFLinkNamesFromVFID(pfName string, vfID int) ([]string, error)
	getPciAddress(ifName string, vf int) (string, error)
}

type pciUtilsImpl struct{}

func (p *pciUtilsImpl) getSriovNumVfs(ifName string) (int, error) {
	return utils.GetSriovNumVfs(ifName)
}

func (p *pciUtilsImpl) getVFLinkNamesFromVFID(pfName string, vfID int) ([]string, error) {
	return utils.GetVFLinkNamesFromVFID(pfName, vfID)
}

func (p *pciUtilsImpl) getPciAddress(ifName string, vf int) (string, error) {
	return utils.GetPciAddress(ifName, vf)
}

// mocked sriovnet interface
// required for unit tests

type Sriovnet interface {
	GetVfRepresentor(string, int) (string, error)
}

type MyLittleSriov struct{}

func (s *MyLittleSriov) GetVfRepresentor(master string, vfid int) (string, error) {
	return sriovnet.GetVfRepresentor(master, vfid)
}

// Manager provides interface invoke sriov nic related operations
type Manager interface {
	SetupVF(conf *types.NetConf, podifName string, cid string, netns ns.NetNS) (string, error)
	ReleaseVF(conf *types.NetConf, podifName string, cid string, netns ns.NetNS) error
	ResetVFConfig(conf *types.NetConf) error
	ApplyVFConfig(conf *types.NetConf) error
	AttachRepresentor(conf *types.NetConf) error
	DetachRepresentor(conf *types.NetConf) error
}

type manager struct {
	nLink NetlinkManager
	utils pciUtils
	sriov Sriovnet
}

// NewManager returns an instance of manager
func NewManager() Manager {
	return &manager{
		nLink: &MyNetlink{},
		utils: &pciUtilsImpl{},
		sriov: &MyLittleSriov{},
	}
}

// SetupVF sets up a VF in Pod netns
func (m *manager) SetupVF(conf *types.NetConf, podifName, cid string, netns ns.NetNS) (string, error) {
	linkName := conf.OrigVfState.HostIFName

	linkObj, err := m.nLink.LinkByName(linkName)
	if err != nil {
		return "", fmt.Errorf("error getting VF netdevice with name %s", linkName)
	}

	// tempName used as intermediary name to avoid name conflicts
	tempName := fmt.Sprintf("%s%d", "temp_", linkObj.Attrs().Index)

	// 1. Set link down
	if err := m.nLink.LinkSetDown(linkObj); err != nil {
		return "", fmt.Errorf("failed to down vf device %q: %v", linkName, err)
	}

	// 2. Set temp name
	if err := m.nLink.LinkSetName(linkObj, tempName); err != nil {
		return "", fmt.Errorf("error setting temp IF name %s for %s", tempName, linkName)
	}

	// 3. Set MAC address
	if conf.MAC != "" {
		hwaddr, err := net.ParseMAC(conf.MAC)
		if err != nil {
			return "", fmt.Errorf("failed to parse MAC address %s: %v", conf.MAC, err)
		}

		// Save the original effective MAC address before overriding it
		conf.OrigVfState.EffectiveMAC = linkObj.Attrs().HardwareAddr.String()

		if err = m.nLink.LinkSetHardwareAddr(linkObj, hwaddr); err != nil {
			return "", fmt.Errorf("failed to set netlink MAC address to %s: %v", hwaddr, err)
		}
	}

	// 4. Change netns
	if err := m.nLink.LinkSetNsFd(linkObj, int(netns.Fd())); err != nil {
		return "", fmt.Errorf("failed to move IF %s to netns: %q", tempName, err)
	}

	var macAddress string

	if err := netns.Do(func(_ ns.NetNS) error {
		// 5. Set Pod IF name
		if err := m.nLink.LinkSetName(linkObj, podifName); err != nil {
			return fmt.Errorf("error setting container interface name %s for %s", linkName, tempName)
		}

		// 6. Bring IF up in Pod netns
		if err := m.nLink.LinkSetUp(linkObj); err != nil {
			return fmt.Errorf("error bringing interface up in container ns: %q", err)
		}

		macAddress = linkObj.Attrs().HardwareAddr.String()
		return nil
	}); err != nil {
		return "", fmt.Errorf("error setting up interface in container namespace: %q", err)
	}
	conf.ContIFNames = podifName

	return macAddress, nil
}

// ReleaseVF reset a VF from Pod netns and return it to init netns
func (m *manager) ReleaseVF(conf *types.NetConf, podifName, cid string, netns ns.NetNS) error {
	initns, err := ns.GetCurrentNS()
	if err != nil {
		return fmt.Errorf("failed to get init netns: %v", err)
	}

	if len(conf.ContIFNames) < 1 && len(conf.ContIFNames) != len(conf.OrigVfState.HostIFName) {
		return fmt.Errorf("number of interface names mismatch ContIFNames: %d HostIFNames: %d",
			len(conf.ContIFNames), len(conf.OrigVfState.HostIFName))
	}

	return netns.Do(func(_ ns.NetNS) error {
		// get VF device
		linkObj, err := m.nLink.LinkByName(podifName)
		if err != nil {
			return fmt.Errorf("failed to get netlink device with name %s: %q", podifName, err)
		}

		// shutdown VF device
		if err = m.nLink.LinkSetDown(linkObj); err != nil {
			return fmt.Errorf("failed to set link %s down: %q", podifName, err)
		}

		// rename VF device
		err = m.nLink.LinkSetName(linkObj, conf.OrigVfState.HostIFName)
		if err != nil {
			return fmt.Errorf("failed to rename link %s to host name %s: %q",
				podifName, conf.OrigVfState.HostIFName, err)
		}

		// reset effective MAC address
		if conf.MAC != "" {
			var hwaddr net.HardwareAddr
			hwaddr, err = net.ParseMAC(conf.OrigVfState.EffectiveMAC)
			if err != nil {
				return fmt.Errorf("failed to parse original effective MAC address %s: %v",
					conf.OrigVfState.EffectiveMAC, err)
			}

			if err = m.nLink.LinkSetHardwareAddr(linkObj, hwaddr); err != nil {
				return fmt.Errorf("failed to restore original effective netlink MAC address %s: %v",
					hwaddr, err)
			}
		}

		// move VF device to init netns
		if err = m.nLink.LinkSetNsFd(linkObj, int(initns.Fd())); err != nil {
			return fmt.Errorf("failed to move interface %s to init netns: %v",
				conf.OrigVfState.HostIFName, err)
		}

		return nil
	})
}

func getVfInfo(link netlink.Link, id int) *netlink.VfInfo {
	attrs := link.Attrs()
	for _, vf := range attrs.Vfs {
		if vf.ID == id {
			return &vf
		}
	}
	return nil
}

// ApplyVFConfig configure a VF with parameters given in NetConf
func (m *manager) ApplyVFConfig(conf *types.NetConf) error {
	pfLink, err := m.nLink.LinkByName(conf.Master)
	if err != nil {
		return fmt.Errorf("failed to lookup master %q: %v", conf.Master, err)
	}

	// Save current the VF state before modifying it
	vfState := getVfInfo(pfLink, conf.VFID)
	if vfState == nil {
		return fmt.Errorf("failed to find vf %d", conf.VFID)
	}
	conf.OrigVfState.FillFromVfInfo(vfState)

	// 1. Set vlan
	if conf.Vlan != nil {
		// set vlan qos if present in the config
		if conf.VlanQoS != nil {
			if err = m.nLink.LinkSetVfVlanQos(pfLink, conf.VFID, *conf.Vlan, *conf.VlanQoS); err != nil {
				return fmt.Errorf("failed to set vf %d vlan configuration: %v", conf.VFID, err)
			}
		} else {
			// set vlan id field only
			if err = m.nLink.LinkSetVfVlan(pfLink, conf.VFID, *conf.Vlan); err != nil {
				return fmt.Errorf("failed to set vf %d vlan: %v", conf.VFID, err)
			}
		}
	}

	// 2. Set mac address
	if conf.MAC != "" {
		var hwaddr net.HardwareAddr
		hwaddr, err = net.ParseMAC(conf.MAC)
		if err != nil {
			return fmt.Errorf("failed to parse MAC address %s: %v", conf.MAC, err)
		}

		if err = m.nLink.LinkSetVfHardwareAddr(pfLink, conf.VFID, hwaddr); err != nil {
			return fmt.Errorf("failed to set MAC address to %s: %v", hwaddr, err)
		}
	}

	// 3. Set min/max tx link rate. 0 means no rate limiting. Support depends on NICs and driver.
	var minTxRate, maxTxRate int
	rateConfigured := false
	if conf.MinTxRate != nil {
		minTxRate = *conf.MinTxRate
		rateConfigured = true
	}

	if conf.MaxTxRate != nil {
		maxTxRate = *conf.MaxTxRate
		rateConfigured = true
	}

	if rateConfigured {
		if err = m.nLink.LinkSetVfRate(pfLink, conf.VFID, minTxRate, maxTxRate); err != nil {
			return fmt.Errorf("failed to set vf %d min_tx_rate to %d Mbps: max_tx_rate to %d Mbps: %v",
				conf.VFID, minTxRate, maxTxRate, err)
		}
	}

	// 4. Set spoofchk flag
	if conf.SpoofChk != "" {
		spoofChk := false
		if conf.SpoofChk == ON {
			spoofChk = true
		}
		if err = m.nLink.LinkSetVfSpoofchk(pfLink, conf.VFID, spoofChk); err != nil {
			return fmt.Errorf("failed to set vf %d spoofchk flag to %s: %v", conf.VFID, conf.SpoofChk, err)
		}
	}

	// 5. Set trust flag
	if conf.Trust != "" {
		trust := false
		if conf.Trust == ON {
			trust = true
		}
		if err = m.nLink.LinkSetVfTrust(pfLink, conf.VFID, trust); err != nil {
			return fmt.Errorf("failed to set vf %d trust flag to %s: %v", conf.VFID, conf.Trust, err)
		}
	}

	// 6. Set link state
	if conf.LinkState != "" {
		var state uint32
		switch conf.LinkState {
		case "auto":
			state = netlink.VF_LINK_STATE_AUTO
		case "enable":
			state = netlink.VF_LINK_STATE_ENABLE
		case "disable":
			state = netlink.VF_LINK_STATE_DISABLE
		default:
			// the value should have been validated earlier, return error if we somehow got here
			return fmt.Errorf("unknown link state %s when setting it for vf %d: %v", conf.LinkState, conf.VFID, err)
		}
		if err = m.nLink.LinkSetVfState(pfLink, conf.VFID, state); err != nil {
			return fmt.Errorf("failed to set vf %d link state to %d: %v", conf.VFID, state, err)
		}
	}

	return nil
}

// ResetVFConfig reset a VF to its original state
func (m *manager) ResetVFConfig(conf *types.NetConf) error {
	pfLink, err := m.nLink.LinkByName(conf.Master)
	if err != nil {
		return fmt.Errorf("failed to lookup master %q: %v", conf.Master, err)
	}

	// Restore VLAN
	if conf.Vlan != nil {
		if conf.VlanQoS != nil {
			if err = m.nLink.LinkSetVfVlanQos(pfLink, conf.VFID, conf.OrigVfState.Vlan, conf.OrigVfState.VlanQoS); err != nil {
				return fmt.Errorf("failed to restore vf %d vlan: %v", conf.VFID, err)
			}
		} else if err = m.nLink.LinkSetVfVlan(pfLink, conf.VFID, conf.OrigVfState.Vlan); err != nil {
			return fmt.Errorf("failed to restore vf %d vlan: %v", conf.VFID, err)
		}
	}

	// Restore spoofchk
	if conf.SpoofChk != "" {
		if err = m.nLink.LinkSetVfSpoofchk(pfLink, conf.VFID, conf.OrigVfState.SpoofChk); err != nil {
			return fmt.Errorf("failed to restore spoofchk for vf %d: %v", conf.VFID, err)
		}
	}

	// Restore the original administrative MAC address
	if conf.MAC != "" {
		var hwaddr net.HardwareAddr
		hwaddr, err = net.ParseMAC(conf.OrigVfState.AdminMAC)
		if err != nil {
			return fmt.Errorf("failed to parse original administrative MAC address %s: %v",
				conf.OrigVfState.AdminMAC, err)
		}
		if err = m.nLink.LinkSetVfHardwareAddr(pfLink, conf.VFID, hwaddr); err != nil {
			return fmt.Errorf("failed to restore original administrative MAC address %s: %v", hwaddr, err)
		}
	}

	// Restore VF trust
	if conf.Trust != "" {
		// TODO: netlink go implementation does not support getting VF trust, need to add support there first
		// for now, just set VF trust to off if it was specified by the user in netconf
		if err = m.nLink.LinkSetVfTrust(pfLink, conf.VFID, false); err != nil {
			return fmt.Errorf("failed to disable trust for vf %d: %v", conf.VFID, err)
		}
	}

	// Restore rate limiting
	if conf.MinTxRate != nil || conf.MaxTxRate != nil {
		err = m.nLink.LinkSetVfRate(pfLink, conf.VFID, conf.OrigVfState.MinTxRate, conf.OrigVfState.MaxTxRate)
		if err != nil {
			return fmt.Errorf("failed to disable rate limiting for vf %d %v", conf.VFID, err)
		}
	}

	// Restore link state to `auto`
	if conf.LinkState != "" {
		// Reset only when link_state was explicitly specified, to  accommodate for drivers / NICs
		// that don't support the netlink command (e.g. igb driver)
		if err = m.nLink.LinkSetVfState(pfLink, conf.VFID, conf.OrigVfState.LinkState); err != nil {
			return fmt.Errorf("failed to set link state to auto for vf %d: %v", conf.VFID, err)
		}
	}

	return nil
}

func (m *manager) AttachRepresentor(conf *types.NetConf) error {
	bridge, err := m.nLink.LinkByName(conf.Bridge)
	if err != nil {
		return fmt.Errorf("failed to get bridge link %s: %v", conf.Bridge, err)
	}

	conf.Representor, err = m.sriov.GetVfRepresentor(conf.Master, conf.VFID)
	if err != nil {
		return fmt.Errorf("failed to get VF's %d representor on NIC %s: %v", conf.VFID, conf.Master, err)
	}

	var rep netlink.Link
	if rep, err = m.nLink.LinkByName(conf.Representor); err != nil {
		return fmt.Errorf("failed to get representor link %s: %v", conf.Representor, err)
	}

	if err = m.nLink.LinkSetUp(rep); err != nil {
		return fmt.Errorf("failed to set representor %s up: %v", conf.Representor, err)
	}

	log.Info().Msgf("Attaching rep %s to the bridge %s", conf.Representor, conf.Bridge)
	return m.nLink.LinkSetMaster(rep, bridge)
}

func (m *manager) DetachRepresentor(conf *types.NetConf) error {
	rep, err := m.nLink.LinkByName(conf.Representor)
	if err != nil {
		return fmt.Errorf("failed to get representor %s link: %v", conf.Representor, err)
	}

	if err = m.nLink.LinkSetDown(rep); err != nil {
		return fmt.Errorf("failed to set representor %s down: %v", conf.Representor, err)
	}

	log.Info().Msgf("Detaching rep %s from the bridge %s", conf.Representor, conf.Bridge)
	return m.nLink.LinkSetNoMaster(rep)
}
