package config

import (
	"encoding/json"
	"fmt"

	"github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/utils"

	localtypes "github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/types"
)

const (
	DefaultBridge = "cni0"
)

// LoadConf load data from stdin to NetConf object
func LoadConf(bytes []byte, netConf *localtypes.NetConf) error {
	netConf.Bridge = DefaultBridge
	if err := json.Unmarshal(bytes, netConf); err != nil {
		return fmt.Errorf("failed to load netconf: %v", err)
	}
	return nil
}

// ParseConf load, parses and validates data from stdin to PluginConf object
func ParseConf(bytes []byte, conf *localtypes.PluginConf) error {
	if err := LoadConf(bytes, &conf.NetConf); err != nil {
		return err
	}

	conf.MAC = conf.NetConf.MAC

	// DeviceID takes precedence; if we are given a VF pciaddr then work from there
	if conf.DeviceID != "" {
		// Get rest of the VF information
		pfName, vfID, err := getVfInfo(conf.DeviceID)
		if err != nil {
			return fmt.Errorf("failed to get VF information: %q", err)
		}
		conf.VFID = vfID
		conf.PFName = pfName
	} else {
		return fmt.Errorf("VF pci addr is required")
	}

	// Assuming VF is netdev interface; Get interface name
	hostIFName, err := utils.GetVFLinkName(conf.DeviceID)
	if err != nil {
		return fmt.Errorf("failed to get VF name: %s", err)
	}
	if hostIFName == "" {
		return fmt.Errorf("VF name is empty")
	}

	conf.OrigVfState.HostIFName = hostIFName

	// validate vlan id range
	if conf.Vlan < 0 || conf.Vlan > 4094 {
		return fmt.Errorf("vlan id %d invalid: value must be in the range 0-4094", conf.Vlan)
	}

	// validate trunk settings
	if len(conf.NetConf.Trunk) > 0 {
		conf.Trunk, err = splitVlanIds(conf.NetConf.Trunk)
		if err != nil {
			return err
		}
	}

	return nil
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
