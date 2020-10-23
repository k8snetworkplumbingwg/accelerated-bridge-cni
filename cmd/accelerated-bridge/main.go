package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ipam"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/vishvananda/netlink"

	"github.com/DmytroLinkin/accelerated-bridge-cni/pkg/config"
	"github.com/DmytroLinkin/accelerated-bridge-cni/pkg/manager"
	"github.com/DmytroLinkin/accelerated-bridge-cni/pkg/utils"
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

func cmdAdd(args *skel.CmdArgs) error {
	netConf, err := config.LoadConf(args.StdinData)
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

	if netConf.Debug {
		setDebugMode()
	}

	envArgs, err := getEnvArgs(args.Args)
	if err != nil {
		return fmt.Errorf("failed to parse args: %v", err)
	}

	if envArgs != nil {
		MAC := string(envArgs.MAC)
		if MAC != "" {
			netConf.MAC = MAC
		}
	}

	// RuntimeConfig takes preference than envArgs.
	// This maintains compatibility of using envArgs
	// for MAC config.
	if netConf.RuntimeConfig.Mac != "" {
		netConf.MAC = netConf.RuntimeConfig.Mac
	}

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", netns, err)
	}
	defer netns.Close()

	m := manager.NewManager()
	if err = m.AttachRepresentor(netConf); err != nil {
		return fmt.Errorf("failed to attach representor: %v", err)
	}
	defer func() {
		if err != nil {
			_ = m.DetachRepresentor(netConf)
		}
	}()

	if err = m.ApplyVFConfig(netConf); err != nil {
		return fmt.Errorf("failed to configure VF %q", err)
	}

	result := &current.Result{}
	result.Interfaces = []*current.Interface{{
		Name:    args.IfName,
		Sandbox: netns.Path(),
	}}

	var macAddr string
	macAddr, err = m.SetupVF(netConf, args.IfName, args.ContainerID, netns)
	defer func() {
		if err != nil {
			err = netns.Do(func(_ ns.NetNS) error {
				_, err = netlink.LinkByName(args.IfName)
				return err
			})
			if err == nil {
				_ = m.ReleaseVF(netConf, args.IfName, args.ContainerID, netns)
			}
		}
	}()
	if err != nil {
		return fmt.Errorf("failed to set up pod interface %q from the device %q: %v",
			args.IfName, netConf.Master, err)
	}

	// run the IPAM plugin
	if netConf.IPAM.Type != "" {
		var r types.Result
		if r, err = ipam.ExecAdd(netConf.IPAM.Type, args.StdinData); err != nil {
			return fmt.Errorf("failed to set up IPAM plugin type %q from the device %q: %v",
				netConf.IPAM.Type, netConf.Master, err)
		}

		defer func() {
			if err != nil {
				_ = ipam.ExecDel(netConf.IPAM.Type, args.StdinData)
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
			return ipam.ConfigureIface(args.IfName, newResult)
		})
		if err != nil {
			return err
		}
		result = newResult
	}

	// Cache NetConf for CmdDel
	if err = utils.SaveNetConf(args.ContainerID, config.DefaultCNIDir, args.IfName, netConf); err != nil {
		return fmt.Errorf("error saving NetConf %q", err)
	}

	return types.PrintResult(result, current.ImplementedSpecVersion)
}

func cmdDel(args *skel.CmdArgs) error {
	// https://github.com/kubernetes/kubernetes/pull/35240
	if args.Netns == "" {
		log.Warn().Msgf("CmdDel skipping - netns is not provided.")
		return nil
	}

	netConf, cRefPath, err := config.LoadConfFromCache(args)
	defer func() {
		if err == nil {
			log.Debug().Msg("CmdDel done.")
		} else {
			log.Error().Msgf("CmdDel failed - %v.", err)
		}
	}()
	if err != nil {
		return err
	}

	if netConf.Debug {
		setDebugMode()
	}

	defer func() {
		if err == nil && cRefPath != "" {
			_ = utils.CleanCachedNetConf(cRefPath)
		}
	}()

	m := manager.NewManager()

	if err = m.DetachRepresentor(netConf); err != nil {
		log.Warn().Msgf("failed to detach representor: %v", err)
	}

	if netConf.IPAM.Type != "" {
		err = ipam.ExecDel(netConf.IPAM.Type, args.StdinData)
		if err != nil {
			return err
		}
	}

	netns, err := ns.GetNS(args.Netns)
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

	if err := m.ReleaseVF(netConf, args.IfName, args.ContainerID, netns); err != nil {
		return err
	}

	if err := m.ResetVFConfig(netConf); err != nil {
		return fmt.Errorf("cmdDel() error reseting VF: %q", err)
	}

	return nil
}

func cmdCheck(args *skel.CmdArgs) error {
	return nil
}

func setupLogger() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: zerolog.TimeFieldFormat,
		NoColor:    true,
	})
}

func setDebugMode() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
}

func main() {
	setupLogger()
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All, "")
}
