package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/vishvananda/netlink"

	"github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/cache"
	"github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/config"
	"github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/manager"
	localtypes "github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/types"
)

//nolint:gochecknoinits
func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

func setDebugMode() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
}

// NewPlugin create and initialize accelerated-bridge-cni Plugin object
func NewPlugin() *Plugin {
	return &Plugin{
		netNS:   &nsWrapper{},
		ipam:    &ipamWrapper{},
		manager: manager.NewManager(),
		config:  config.NewConfig(),
		cache:   cache.NewStateCache(),
	}
}

// Plugin is accelerated-bridge-cni implementation
type Plugin struct {
	netNS   NS
	ipam    IPAM
	manager manager.Manager
	config  config.Loader
	cache   cache.StateCache
}

// CmdAdd implementation of accelerated-bridge-cni plugin
func (p *Plugin) CmdAdd(args *skel.CmdArgs) error {
	var err error

	defer func() {
		if err == nil {
			log.Debug().Msg("CmdAdd done.")
		}
	}()

	cmdCtx := &cmdContext{
		args:       args,
		pluginConf: &localtypes.PluginConf{},
		result:     &current.Result{},
	}
	defer func() {
		cmdCtx.handleError(err)
	}()

	cmdCtx.registerErrorHandler(func() {
		log.Error().Msgf("CmdAdd failed - %v.", err)
	})

	err = p.config.ParseConf(args.StdinData, cmdCtx.pluginConf)
	if err != nil {
		return fmt.Errorf("failed to load netconf: %v", err)
	}

	pluginConf := cmdCtx.pluginConf
	if pluginConf.Debug {
		setDebugMode()
	}

	cmdCtx.netNS, err = p.netNS.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", args.Netns, err)
	}
	defer cmdCtx.netNS.Close()

	cmdCtx.result.Interfaces = []*current.Interface{{
		Name:    args.IfName,
		Sandbox: cmdCtx.netNS.Path(),
	}}

	err = p.getMACAddressConfig(cmdCtx)
	if err != nil {
		return fmt.Errorf("failed to get MAC config: %v", err)
	}

	if err = p.manager.AttachRepresentor(pluginConf); err != nil {
		return fmt.Errorf("failed to attach representor: %v", err)
	}
	cmdCtx.registerErrorHandler(func() {
		_ = p.manager.DetachRepresentor(pluginConf)
	})

	if err = p.manager.ApplyVFConfig(pluginConf); err != nil {
		return fmt.Errorf("failed to configure VF %q", err)
	}

	var macAddr string
	if !pluginConf.IsUserspaceDriver {
		macAddr, err = p.manager.SetupVF(pluginConf, args.IfName, args.ContainerID, cmdCtx.netNS)
		cmdCtx.registerErrorHandler(func() {
			netNSErr := cmdCtx.netNS.Do(func(_ ns.NetNS) error {
				_, intErr := netlink.LinkByName(args.IfName)
				return intErr
			})
			if netNSErr == nil {
				_ = p.manager.ReleaseVF(pluginConf, args.IfName, args.ContainerID, cmdCtx.netNS)
			}
		})

		if err != nil {
			return fmt.Errorf("failed to set up pod interface %q from the device %q: %v",
				args.IfName, pluginConf.PFName, err)
		}
	}

	// run the IPAM plugin
	if pluginConf.IPAM.Type != "" {
		err = p.configureIPAM(cmdCtx, macAddr)
		if err != nil {
			return fmt.Errorf("failed to configure IPAM: %v", err)
		}
	}
	// Cache PluginConf for CmdDel
	pRef := p.cache.GetStateRef(pluginConf.Name, args.ContainerID, args.IfName)
	if err = p.cache.Save(pRef, pluginConf); err != nil {
		return fmt.Errorf("failed to save PluginConf %q", err)
	}

	if err = p.updateDeviceInfo(cmdCtx); err != nil {
		log.Error().Msgf("failed to update DeviceInfo %v.", err)
		// this step is not critical for CNI operation, log error and continue
	}
	return types.PrintResult(cmdCtx.result, pluginConf.CNIVersion)
}

// updateDeviceInfo updates CNIDeviceInfoFile file with information
// about VF representor
func (p *Plugin) updateDeviceInfo(cmdCtx *cmdContext) error {
	if cmdCtx.pluginConf.RuntimeConfig.CNIDeviceInfoFile == "" {
		return nil
	}
	versionKey := "version"
	pciDevInfoKey := "pci"
	vfRepresentorNameKey := "representor-device"

	// if spec in DeviceInfo file has spec version "1.0.0", then the plugin
	// will update spec version to "1.1.0" (minimal required spec version for representor-device field),
	// if spec version != "1.0.0", then existing spec version field in
	// DeviceInfo file will be preserved
	unsupportedSpecVersion := "1.0.0"
	minimalSpecVersion := "1.1.0"

	// we use map[string]interface{}{} to unmarshal DeviceInfo JSON to avoid importing types
	// from network-attach-definition-client package which will introduce many additional
	// dependencies
	devInfo := map[string]interface{}{}

	bytes, err := os.ReadFile(cmdCtx.pluginConf.RuntimeConfig.CNIDeviceInfoFile)
	if err != nil {
		return fmt.Errorf("failed to read CNIDeviceInfoFile %q", err)
	}
	err = json.Unmarshal(bytes, &devInfo)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CNIDeviceInfoFile %q", err)
	}

	versionField, exist := devInfo[versionKey]
	if !exist {
		return fmt.Errorf("failed to detect DeviceInfo spec version in CNIDeviceInfoFile: no version field")
	}
	currentVersion, ok := versionField.(string)
	if !ok {
		return fmt.Errorf("unexpected version field format in CNIDeviceInfoFile")
	}
	if currentVersion == unsupportedSpecVersion {
		// should upgrade version to minimalSpecVersion
		log.Debug().Msgf(
			"update DeviceInfo spec version in CNIDeviceInfoFile to %s version", minimalSpecVersion)
		devInfo[versionKey] = minimalSpecVersion
	}

	pciInfo, exist := devInfo[pciDevInfoKey]
	if !exist {
		return fmt.Errorf("pci field not found in CNIDeviceInfoFile")
	}
	pciInfoMap, ok := pciInfo.(map[string]interface{})
	if !ok {
		return fmt.Errorf("unexpected pci info format in CNIDeviceInfoFile")
	}
	currentValue := pciInfoMap[vfRepresentorNameKey]
	if currentValue == cmdCtx.pluginConf.Representor {
		// preserve existing value if already set
		log.Debug().Msgf("representor-device already set, skip deviceInfo update")
		return nil
	}
	pciInfoMap[vfRepresentorNameKey] = cmdCtx.pluginConf.Representor
	devInfo[pciDevInfoKey] = pciInfoMap

	bytes, err = json.Marshal(&devInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal DeviceInfo to JSON %q", err)
	}
	//nolint:gosec
	// save with same permissions as https://github.com/k8snetworkplumbingwg/network-attachment-definition-client
	// utils.saveDeviceInfo
	err = os.WriteFile(cmdCtx.pluginConf.RuntimeConfig.CNIDeviceInfoFile, bytes, 0444)
	if err != nil {
		return fmt.Errorf("failed to update CNIDeviceInfoFile %q", err)
	}
	return nil
}

// mac address configuration can be supplied as
// top-level configuration option in cni conf,
// as env variable and in RuntimeConfig section in cni conf
// priority: 1. runtime config 2. env 3. top-level option
func (p *Plugin) getMACAddressConfig(cmdCtx *cmdContext) error {
	envArgs, err := getEnvArgs(cmdCtx.args.Args)
	if err != nil {
		return fmt.Errorf("failed to parse args: %v", err)
	}
	pluginConf := cmdCtx.pluginConf
	if envArgs != nil {
		MAC := string(envArgs.MAC)
		if MAC != "" {
			pluginConf.MAC = MAC
		}
	}
	// RuntimeConfig takes preference than envArgs.
	// This maintains compatibility of using envArgs
	// for MAC config.
	if pluginConf.RuntimeConfig.Mac != "" {
		pluginConf.MAC = pluginConf.RuntimeConfig.Mac
	}
	return nil
}

// call ipam plugin
func (p *Plugin) configureIPAM(cmdCtx *cmdContext, macAddr string) error {
	var ipamResult types.Result
	var err error

	pluginConf := cmdCtx.pluginConf
	args := cmdCtx.args

	if ipamResult, err = p.ipam.ExecAdd(pluginConf.IPAM.Type, args.StdinData); err != nil {
		return fmt.Errorf("failed to set up IPAM plugin type %q from the device %q: %v",
			pluginConf.IPAM.Type, pluginConf.PFName, err)
	}

	cmdCtx.registerErrorHandler(func() {
		_ = p.ipam.ExecDel(pluginConf.IPAM.Type, args.StdinData)
	})

	// Convert the IPAM result into the current Result type
	var newResult *current.Result
	if newResult, err = current.NewResultFromResult(ipamResult); err != nil {
		return err
	}

	if len(newResult.IPs) == 0 {
		err = errors.New("IPAM plugin returned missing IP config")
		return err
	}

	newResult.Interfaces = cmdCtx.result.Interfaces
	newResult.Interfaces[0].Mac = macAddr

	for _, ipc := range newResult.IPs {
		// All addresses apply to the container interface (move from host)
		ipc.Interface = current.Int(0)
	}

	if !pluginConf.IsUserspaceDriver {
		err = cmdCtx.netNS.Do(func(_ ns.NetNS) error {
			return p.ipam.ConfigureIface(args.IfName, newResult)
		})
		if err != nil {
			return err
		}
	}
	cmdCtx.result = newResult
	return nil
}

// CmdDel implementation of accelerated-bridge-cni plugin
func (p *Plugin) CmdDel(args *skel.CmdArgs) error {
	// https://github.com/kubernetes/kubernetes/pull/35240
	if args.Netns == "" {
		log.Warn().Msgf("CmdDel skipping - netns is not provided.")
		return nil
	}

	var err error
	defer func() {
		if err == nil {
			log.Debug().Msg("CmdDel done.")
		} else {
			log.Error().Msgf("CmdDel failed - %v.", err)
		}
	}()

	netConf := &localtypes.NetConf{}
	err = p.config.LoadConf(args.StdinData, netConf)
	if err != nil {
		return err
	}

	pRef := p.cache.GetStateRef(netConf.Name, args.ContainerID, args.IfName)

	pluginConf := &localtypes.PluginConf{}
	err = p.cache.Load(pRef, pluginConf)
	if err != nil {
		// If cmdDel() fails, cached netconf is cleaned up by
		// the followed defer call or might not exist in the first place.
		// However, subsequence calls of cmdDel()
		// from container runtime fail in a dead loop because
		// the cached netconf doesn't exist.
		// Return nil when cache.Load() fails since the rest
		// of cmdDel() code relies on netconf as input argument
		// and there is no meaning to continue.
		return nil
	}

	if pluginConf.Debug {
		setDebugMode()
	}

	defer func() {
		if err == nil {
			_ = p.cache.Delete(pRef)
		}
	}()

	if err = p.manager.DetachRepresentor(pluginConf); err != nil {
		log.Warn().Msgf("failed to detach representor: %v", err)
	}

	if pluginConf.IPAM.Type != "" {
		err = p.ipam.ExecDel(pluginConf.IPAM.Type, args.StdinData)
		if err != nil {
			return err
		}
	}

	netns, err := p.netNS.GetNS(args.Netns)
	if err != nil {
		// according to:
		// https://github.com/kubernetes/kubernetes/issues/43014#issuecomment-287164444
		// if provided path does not exist (e.x. when node was restarted)
		// plugin should silently return with success after releasing
		// IPAM resources
		_, ok := err.(ns.NSPathNotExistErr)
		if ok {
			return nil
		}

		return fmt.Errorf("failed to open netns %s: %q", netns, err)
	}
	defer netns.Close()

	if !pluginConf.IsUserspaceDriver {
		//nolint:gocritic
		if err = p.manager.ReleaseVF(pluginConf, args.IfName, args.ContainerID, netns); err != nil {
			return err
		}
	}

	//nolint:gocritic
	if err = p.manager.ResetVFConfig(pluginConf); err != nil {
		return fmt.Errorf("cmdDel() error reseting VF: %q", err)
	}

	return nil
}

// CmdCheck implementation of accelerated-bridge-cni plugin
func (p *Plugin) CmdCheck(args *skel.CmdArgs) error {
	return nil
}
