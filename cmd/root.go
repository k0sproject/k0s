package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/k0sproject/k0s/pkg/build"
	"github.com/k0sproject/k0s/pkg/util"
)

var cfgFile string
var debug bool

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/k0s.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Debug logging (default is false)")

	// initialize configuration
	err := initConfig()
	if err != nil {
		fmt.Printf("err: %v", err)
	}

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(tokenCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(workerCmd)
	rootCmd.AddCommand(APICmd)
	rootCmd.AddCommand(etcdCmd)
}

var (
	rootCmd = &cobra.Command{
		Use:   "k0s",
		Short: "k0s - Zero Friction Kubernetes",
		Long: `k0s is yet another Kubernetes distro. Yes. But we do some of the things pretty different from other distros out there.
It is a single binary all-inclusive Kubernetes distribution with all the required bells and whistles preconfigured to make 
building a Kubernetes clusters a matter of just copying an executable to every host and running it.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// set DEBUG from env, or from command flag
			if viper.GetString("debug") != "" || debug {
				logrus.SetLevel(logrus.DebugLevel)
			}
		},
	}

	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the k0s version",

		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(build.Version)
		},
	}
)

func initConfig() error {
	if cfgFile == "" {
		home, err := util.HomeDir()
		if err != nil {
			return err
		}
		cfgFile = filepath.Join(home, "k0s.yaml")
	}
	// check if config file exists
	if util.FileExists(cfgFile) {
		viper.SetConfigFile(cfgFile)
		logrus.Debugf("Using config file: %v", cfgFile)
	}
	// Add env vars to Config
	viper.AutomaticEnv()
	return nil
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		log.Fatal(err)
	}
}
