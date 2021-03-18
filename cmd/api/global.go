package api

import (
	"fmt"

	"github.com/spf13/pflag"

	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
)

var (
	cfgFile       string
	dataDir       string
	debug         bool
	debugListenOn string
	k0sVars       constant.CfgVars
)

func getPersistentFlagSet() *pflag.FlagSet {
	flagset := &pflag.FlagSet{}
	flagset.StringVarP(&cfgFile, "config", "c", "", "config file (default: ./k0s.yaml)")
	flagset.BoolVarP(&debug, "debug", "d", false, "Debug logging (default: false)")
	flagset.StringVar(&dataDir, "data-dir", "", "Data Directory for k0s (default: /var/lib/k0s). DO NOT CHANGE for an existing setup, things will break!")
	flagset.StringVar(&debugListenOn, "debugListenOn", ":6060", "Http listenOn for debug pprof handler")
	return flagset
}

func getCmdOpts() CmdOpts {
	k0sVars = constant.GetConfig(dataDir)
	dvalue := config.GetBool(debug)

	opts := CmdOpts{
		CfgFile:          fmt.Sprintf("%s", cfgFile),
		Debug:            dvalue,
		DefaultLogLevels: config.DefaultLogLevels(),
		K0sVars:          k0sVars,
	}
	return opts
}
