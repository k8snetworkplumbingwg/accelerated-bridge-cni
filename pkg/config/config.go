package config

import (
	"encoding/json"
	"fmt"
	"strings"

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
	return &Config{
		sriovnetProvider: &utils.SriovnetWrapper{},
		netlink:          &utils.NetlinkWrapper{},
	}
}

// Config provides function to load and parse cni configuration
type Config struct {
	sriovnetProvider utils.SriovnetProvider
	netlink          utils.Netlink
}

// LoadConf load data from stdin to NetConf object
func (c *Config) LoadConf(bytes []byte, netConf *localtypes.NetConf) error {
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

	err = c.handleBridgeConfig(conf)
	if err != nil {
		return err
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

// handleBridgeConfig checks CNI bridge configuration and set ActualBridge options for PluginConfig.
// If config.Bridge option is empty, config.ActualBridge will be the value of DefaultBridge const.
// If config.Bridge option contains one bridge name, config.ActualBridge will be that bridge.
// If config.Bridge option contains a list of bridges, then auto-detect logic will be used,
// and the function will iterate over configured bridges and check to which bridge uplink is connected.
// Uplink should be added to one of the bridges when multiple bridges are set in config,
// this is required for auto-detect logic.
// When a single bridge is specified in plugin configuration there will be no validation that
// uplink is a part of a bridge, this is required for backward compatibility.
func (c *Config) handleBridgeConfig(conf *localtypes.PluginConf) error {
	if conf.Bridge == "" {
		conf.Bridge = DefaultBridge
	}
	bridgeNamesInConf := strings.Split(conf.Bridge, ",")
	allowedBridgeNames := make([]string, 0, len(bridgeNamesInConf))
	for _, brName := range bridgeNamesInConf {
		brName = strings.TrimSpace(brName)
		if brName == "" {
			return fmt.Errorf("bridge configuration option has invalid format")
		}
		allowedBridgeNames = append(allowedBridgeNames, brName)
	}
	if len(allowedBridgeNames) == 0 {
		return fmt.Errorf("bridge configuration option has invalid format")
	}

	if len(allowedBridgeNames) == 1 {
		// single bridge in config, skip bridge auto detect logic
		conf.ActualBridge = allowedBridgeNames[0]
		return nil
	}

	pfLink, err := c.netlink.LinkByName(conf.PFName)
	if err != nil {
		return fmt.Errorf("failed to get link info for uplink %s: %q", conf.PFName, err)
	}

	pfBridgeLink, err := utils.GetParentBridgeForLink(c.netlink, pfLink)
	if err != nil {
		return fmt.Errorf("failed to get parent bridge for uplink %s: %q", conf.PFName, err)
	}

	for _, allowedBridge := range allowedBridgeNames {
		if pfBridgeLink.Attrs().Name == allowedBridge {
			conf.ActualBridge = pfBridgeLink.Attrs().Name
			break
		}
	}

	if conf.ActualBridge == "" {
		return fmt.Errorf("uplink %s is attached to %s bridge, "+
			"expected to be attached to one of the following bridges: %q",
			conf.PFName, pfBridgeLink.Attrs().Name, allowedBridgeNames)
	}
	return nil
}
