package config

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/containernetworking/cni/pkg/skel"

	localtypes "github.com/Mellanox/accelerated-bridge-cni/pkg/types"
	"github.com/Mellanox/accelerated-bridge-cni/pkg/utils"
)

const (
	// DefaultCNIDir used for caching PluginConf
	DefaultCNIDir = "/var/lib/cni/accelerated-bridge"
	DefaultBridge = "cni0"
)

// LoadConf parses and validates stdin netconf and returns PluginConf object
func LoadConf(bytes []byte) (*localtypes.PluginConf, error) {
	conf := &localtypes.PluginConf{
		NetConf: localtypes.NetConf{
			Debug:  false,
			Bridge: DefaultBridge,
		}}
	if err := json.Unmarshal(bytes, &conf.NetConf); err != nil {
		return nil, fmt.Errorf("failed to load netconf: %v", err)
	}

	conf.MAC = conf.NetConf.MAC

	// DeviceID takes precedence; if we are given a VF pciaddr then work from there
	if conf.DeviceID != "" {
		// Get rest of the VF information
		pfName, vfID, err := getVfInfo(conf.DeviceID)
		if err != nil {
			return nil, fmt.Errorf("failed to get VF information: %q", err)
		}
		conf.VFID = vfID
		conf.PFName = pfName
	} else {
		return nil, fmt.Errorf("VF pci addr is required")
	}

	// Assuming VF is netdev interface; Get interface name
	hostIFName, err := utils.GetVFLinkName(conf.DeviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get VF name: %s", err)
	}
	if hostIFName == "" {
		return nil, fmt.Errorf("VF name is empty")
	}

	conf.OrigVfState.HostIFName = hostIFName

	// validate vlan id range
	if conf.Vlan < 0 || conf.Vlan > 4094 {
		return nil, fmt.Errorf("vlan id %d invalid: value must be in the range 0-4094", conf.Vlan)
	}

	// validate trunk settings
	if len(conf.NetConf.Trunk) > 0 {
		conf.Trunk, err = splitVlanIds(conf.NetConf.Trunk)
		if err != nil {
			return nil, err
		}
	}

	return conf, nil
}

// SaveConf serialize and save PluginConf to cache
func SaveConf(conf *localtypes.PluginConf, args *skel.CmdArgs) error {
	data, err := json.Marshal(conf)
	if err != nil {
		return fmt.Errorf("failed to serialize CNI conf: %s", err)
	}
	if err = utils.SaveNetConf(args.ContainerID, DefaultCNIDir, args.IfName, data); err != nil {
		return fmt.Errorf("error saving PluginConf %q", err)
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

// LoadConfFromCache retrieves cached PluginConf returns it along with a handle for removal
func LoadConfFromCache(args *skel.CmdArgs) (*localtypes.PluginConf, string, error) {
	netConf := &localtypes.PluginConf{}

	s := []string{args.ContainerID, args.IfName}
	cRef := strings.Join(s, "-")
	cRefPath := filepath.Join(DefaultCNIDir, cRef)

	netConfBytes, err := utils.ReadScratchNetConf(cRefPath)
	if err != nil {
		return nil, "", fmt.Errorf("error reading cached PluginConf in %s with name %s", DefaultCNIDir, cRef)
	}

	if err = json.Unmarshal(netConfBytes, netConf); err != nil {
		return nil, "", fmt.Errorf("failed to parse PluginConf: %q", err)
	}

	return netConf, cRefPath, nil
}
