package plugin

import (
	"github.com/containernetworking/cni/pkg/skel"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ns"

	localtypes "github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/types"
)

// cmdContext struct holds context of the current command call
type cmdContext struct {
	args       *skel.CmdArgs
	pluginConf *localtypes.PluginConf

	netNS  ns.NetNS
	result *current.Result

	errorHandlers []func()
}

// add register cleanup function which should be called if cmd completed with error
func (cmd *cmdContext) registerErrorHandler(f func()) {
	cmd.errorHandlers = append(cmd.errorHandlers, f)
}

// run registered clean up functions if command completed with error
func (cmd *cmdContext) handleError(err error) {
	if err != nil {
		for i := len(cmd.errorHandlers) - 1; i >= 0; i-- {
			cmd.errorHandlers[i]()
		}
	}
}
