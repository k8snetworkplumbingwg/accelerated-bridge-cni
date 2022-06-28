package config

import (
	"encoding/json"
	"fmt"

	localtypes "github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/types"
	"github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/utils"
)

const (
	DefaultBridge = "cni0"
)

type Loader interface {
	LoadConf(bytes []byte, netConf *localtypes.NetConf) error
	ParseConf(bytes []byte, conf *localtypes.PluginConf) error
}

// NewConfig create and initialize Config struct
func NewConfig() *Config {
	return &Config{sriovnetProvider: &utils.SriovnetWrapper{}}
}

// Config provides function to load and parse cni configuration
type Config struct {
	sriovnetProvider utils.SriovnetProvider
}

// LoadConf load data from stdin to NetConf object
func (c *Config) LoadConf(bytes []byte, netConf *localtypes.NetConf) error {
	netConf.Bridge = DefaultBridge
	if err := json.Unmarshal(bytes, netConf); err != nil {
		return fmt.Errorf("failed to load netconf: %v", err)
	}
	return nil
}

// ParseConf load, parses and validates data from stdin to PluginConf object
func (c *Config) ParseConf(bytes []byte, conf *localtypes.PluginConf) error {
	if err := c.LoadConf(bytes, &conf.NetConf); err != nil {
		return err
	}

	conf.MAC = conf.NetConf.MAC
	conf.MTU = conf.NetConf.MTU

	// DeviceID takes precedence; if we are given a VF pciaddr then work from there
	if conf.DeviceID == "" {
		return fmt.Errorf("VF pci addr is required")
	}

	// Get rest of the VF information
	var err error
	conf.PFName, conf.VFID, err = c.getVfInfo(conf.DeviceID)
	if err != nil {
		return fmt.Errorf("failed to get VF information: %q", err)
	}

	// Assuming VF is netdev interface; Get interface name
	hostIFName, err := utils.GetVFLinkName(conf.DeviceID)
	if err != nil || hostIFName == "" {
		conf.IsUserspaceDriver, err = utils.HasUserspaceDriver(conf.DeviceID)
		if err != nil {
			return fmt.Errorf("failed to detect if VF %s has userspace driver %q", conf.DeviceID, err)
		}
		if !conf.IsUserspaceDriver {
			return fmt.Errorf("the VF %s does not have a interface name or a userspace driver", conf.DeviceID)
		}
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

func (c *Config) getVfInfo(vfPci string) (string, int, error) {
	var vfID int

	pf, err := c.sriovnetProvider.GetUplinkRepresentor(vfPci)
	if err != nil {
		return "", vfID, err
	}

	vfID, err = utils.GetVfid(vfPci, pf)
	if err != nil {
		return "", vfID, err
	}

	return pf, vfID, nil
}
