package plugin

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
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

	return types.PrintResult(cmdCtx.result, current.ImplementedSpecVersion)
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
		return err
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
