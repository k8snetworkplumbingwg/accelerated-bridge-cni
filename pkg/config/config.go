package config

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/containernetworking/cni/pkg/skel"

	localtypes "github.com/DmytroLinkin/accelerated-bridge-cni/pkg/types"
	"github.com/DmytroLinkin/accelerated-bridge-cni/pkg/utils"
)

const (
	// DefaultCNIDir used for caching NetConf
	DefaultCNIDir = "/var/lib/cni/accelerated-bridge"
	DefaultBridge = "cni0"
)

// LoadConf parses and validates stdin netconf and returns NetConf object
func LoadConf(bytes []byte) (*localtypes.NetConf, error) {
	n := &localtypes.NetConf{
		Debug:  false,
		Bridge: DefaultBridge,
	}
	if err := json.Unmarshal(bytes, n); err != nil {
		return nil, fmt.Errorf("LoadConf(): failed to load netconf: %v", err)
	}

	// DeviceID takes precedence; if we are given a VF pciaddr then work from there
	if n.DeviceID != "" {
		// Get rest of the VF information
		pfName, vfID, err := getVfInfo(n.DeviceID)
		if err != nil {
			return nil, fmt.Errorf("LoadConf(): failed to get VF information: %q", err)
		}
		n.VFID = vfID
		n.Master = pfName
	} else {
		return nil, fmt.Errorf("LoadConf(): VF pci addr is required")
	}

	// Assuming VF is netdev interface; Get interface name
	hostIFName, err := utils.GetVFLinkName(n.DeviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get VF name: %s", err)
	}
	if hostIFName == "" {
		return nil, fmt.Errorf("VF name is empty")
	}

	n.OrigVfState.HostIFName = hostIFName

	if n.Vlan != nil {
		// validate vlan id range
		if *n.Vlan < 0 || *n.Vlan > 4094 {
			return nil, fmt.Errorf("LoadConf(): vlan id %d invalid: value must be in the range 0-4094", *n.Vlan)
		}
	}

	if n.VlanQoS != nil {
		// validate that VLAN QoS is in the 0-7 range
		if *n.VlanQoS < 0 || *n.VlanQoS > 7 {
			return nil, fmt.Errorf("LoadConf(): vlan QoS PCP %d invalid: value must be in the range 0-7", *n.VlanQoS)
		}
		// validate that vlan id is set
		if n.Vlan == nil {
			return nil, fmt.Errorf("LoadConf(): vlan id must be configured to set vlan QoS")
		}
		// validate non-zero value for vlan id if vlan qos is set to a non-zero value
		if *n.VlanQoS != 0 && *n.Vlan == 0 {
			return nil, fmt.Errorf("LoadConf(): non-zero vlan id must be configured to set vlan QoS to a non-zero value")
		}
	}

	// validate that link state is one of supported values
	if n.LinkState != "" && n.LinkState != "auto" && n.LinkState != "enable" && n.LinkState != "disable" {
		return nil, fmt.Errorf("LoadConf(): invalid link_state value: %s", n.LinkState)
	}

	return n, nil
}

func getVfInfo(vfPci string) (string, int, error) {
	var vfID int

	pf, err := utils.GetPfName(vfPci)
	if err != nil {
		return "", vfID, err
	}

	vfID, err = utils.GetVfid(vfPci, pf)
	if err != nil {
		return "", vfID, err
	}

	return pf, vfID, nil
}

// LoadConfFromCache retrieves cached NetConf returns it along with a handle for removal
func LoadConfFromCache(args *skel.CmdArgs) (*localtypes.NetConf, string, error) {
	netConf := &localtypes.NetConf{}

	s := []string{args.ContainerID, args.IfName}
	cRef := strings.Join(s, "-")
	cRefPath := filepath.Join(DefaultCNIDir, cRef)

	netConfBytes, err := utils.ReadScratchNetConf(cRefPath)
	if err != nil {
		return nil, "", fmt.Errorf("error reading cached NetConf in %s with name %s", DefaultCNIDir, cRef)
	}

	if err = json.Unmarshal(netConfBytes, netConf); err != nil {
		return nil, "", fmt.Errorf("failed to parse NetConf: %q", err)
	}

	return netConf, cRefPath, nil
}
