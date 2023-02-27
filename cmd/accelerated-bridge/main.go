package main

import (
	"os"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/k8snetworkplumbingwg/accelerated-bridge-cni/pkg/plugin"
)

func setupLogger() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: zerolog.TimeFieldFormat,
		NoColor:    true,
	})
}

func main() {
	setupLogger()
	p := plugin.NewPlugin()
	skel.PluginMain(p.CmdAdd, p.CmdCheck, p.CmdDel,
		version.PluginSupports("0.1.0", "0.2.0", "0.3.0", "0.3.1", "0.4.0"), "")
}
