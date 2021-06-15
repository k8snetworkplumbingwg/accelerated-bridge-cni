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

type envArgs struct {
	types.CommonArgs
	MAC types.UnmarshallableString `json:"mac,omitempty"`
}

//nolint:gochecknoinits
func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

func getEnvArgs(envArgsString string) (*envArgs, error) {
	if envArgsString != "" {
		e := envArgs{}
		err := types.LoadArgs(envArgsString, &e)
		if err != nil {
			return nil, err
		}
		return &e, nil
	}
	return nil, nil
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
		cache:   cache.NewStateCache(),
	}
}

// Plugin is accelerated-bridge-cni implementation
type Plugin struct {
	netNS   NS
	ipam    IPAM
	manager manager.Manager
	cache   cache.StateCache
}

// CmdAdd implementation of accelerated-bridge-cni plugin
func (p *Plugin) CmdAdd(args *skel.CmdArgs) error {
	pluginConf := &localtypes.PluginConf{}
	err := config.ParseConf(args.StdinData, pluginConf)
	defer func() {
		if err == nil {
			log.Debug().Msg("CmdAdd done.")
		} else {
			log.Error().Msgf("CmdAdd failed - %v.", err)
		}
	}()
	if err != nil {
		return fmt.Errorf("failed to load netconf: %v", err)
	}

	if pluginConf.Debug {
		setDebugMode()
	}

	envArgs, err := getEnvArgs(args.Args)
	if err != nil {
		return fmt.Errorf("failed to parse args: %v", err)
	}

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

	netns, err := p.netNS.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", netns, err)
	}
	defer netns.Close()

	if err = p.manager.AttachRepresentor(pluginConf); err != nil {
		return fmt.Errorf("failed to attach representor: %v", err)
	}
	defer func() {
		if err != nil {
			_ = p.manager.DetachRepresentor(pluginConf)
		}
	}()

	if err = p.manager.ApplyVFConfig(pluginConf); err != nil {
		return fmt.Errorf("failed to configure VF %q", err)
	}

	result := &current.Result{}
	result.Interfaces = []*current.Interface{{
		Name:    args.IfName,
		Sandbox: netns.Path(),
	}}

	var macAddr string
	macAddr, err = p.manager.SetupVF(pluginConf, args.IfName, args.ContainerID, netns)
	defer func() {
		if err != nil {
			err = netns.Do(func(_ ns.NetNS) error {
				_, err = netlink.LinkByName(args.IfName)
				return err
			})
			if err == nil {
				_ = p.manager.ReleaseVF(pluginConf, args.IfName, args.ContainerID, netns)
			}
		}
	}()
	if err != nil {
		return fmt.Errorf("failed to set up pod interface %q from the device %q: %v",
			args.IfName, pluginConf.PFName, err)
	}

	// run the IPAM plugin
	if pluginConf.IPAM.Type != "" {
		var r types.Result
		if r, err = p.ipam.ExecAdd(pluginConf.IPAM.Type, args.StdinData); err != nil {
			return fmt.Errorf("failed to set up IPAM plugin type %q from the device %q: %v",
				pluginConf.IPAM.Type, pluginConf.PFName, err)
		}

		defer func() {
			if err != nil {
				_ = p.ipam.ExecDel(pluginConf.IPAM.Type, args.StdinData)
			}
		}()

		// Convert the IPAM result into the current Result type
		var newResult *current.Result
		if newResult, err = current.NewResultFromResult(r); err != nil {
			return err
		}

		if len(newResult.IPs) == 0 {
			return errors.New("IPAM plugin returned missing IP config")
		}

		newResult.Interfaces = result.Interfaces
		newResult.Interfaces[0].Mac = macAddr

		for _, ipc := range newResult.IPs {
			// All addresses apply to the container interface (move from host)
			ipc.Interface = current.Int(0)
		}

		err = netns.Do(func(_ ns.NetNS) error {
			return p.ipam.ConfigureIface(args.IfName, newResult)
		})
		if err != nil {
			return err
		}
		result = newResult
	}

	// Cache PluginConf for CmdDel
	pRef := p.cache.GetStateRef(pluginConf.Name, args.ContainerID, args.IfName)
	if err = p.cache.Save(pRef, &pluginConf); err != nil {
		return fmt.Errorf("failed to save PluginConf %q", err)
	}

	return types.PrintResult(result, current.ImplementedSpecVersion)
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
	err = config.LoadConf(args.StdinData, netConf)
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

	if err := p.manager.ReleaseVF(pluginConf, args.IfName, args.ContainerID, netns); err != nil {
		return err
	}

	if err := p.manager.ResetVFConfig(pluginConf); err != nil {
		return fmt.Errorf("cmdDel() error reseting VF: %q", err)
	}

	return nil
}

// CmdCheck implementation of accelerated-bridge-cni plugin
func (p *Plugin) CmdCheck(args *skel.CmdArgs) error {
	return nil
}
