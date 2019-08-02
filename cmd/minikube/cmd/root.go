/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	goflag "flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/docker/machine/libmachine"
	"github.com/docker/machine/libmachine/log"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/kubectl/pkg/util/templates"
	configCmd "k8s.io/minikube/cmd/minikube/cmd/config"
	"k8s.io/minikube/pkg/minikube/bootstrapper"
	"k8s.io/minikube/pkg/minikube/bootstrapper/kubeadm"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/pkg/minikube/exit"
	"k8s.io/minikube/pkg/minikube/notify"
	"k8s.io/minikube/pkg/minikube/translate"
)

var dirs = [...]string{
	constants.GetMinipath(),
	constants.MakeMiniPath("certs"),
	constants.MakeMiniPath("machines"),
	constants.MakeMiniPath("cache"),
	constants.MakeMiniPath("cache", "iso"),
	constants.MakeMiniPath("config"),
	constants.MakeMiniPath("addons"),
	constants.MakeMiniPath("files"),
	constants.MakeMiniPath("logs"),
}

var (
	enableUpdateNotification = true
)

var viperWhiteList = []string{
	"v",
	"alsologtostderr",
	"log_dir",
}

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "minikube",
	Short: "Minikube is a tool for managing local Kubernetes clusters.",
	Long:  `Minikube is a CLI tool that provisions and manages single-node Kubernetes clusters optimized for development workflows.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		for _, path := range dirs {
			if err := os.MkdirAll(path, 0777); err != nil {
				exit.WithError("Error creating minikube directory", err)
			}
		}

		// Log level 3 or greater enables libmachine logs
		if !glog.V(3) {
			log.SetOutWriter(ioutil.Discard)
			log.SetErrWriter(ioutil.Discard)
		}

		// Log level 7 or greater enables debug level logs
		if glog.V(7) {
			log.SetDebug(true)
		}

		logDir := pflag.Lookup("log_dir")
		if !logDir.Changed {
			if err := logDir.Value.Set(constants.MakeMiniPath("logs")); err != nil {
				exit.WithError("logdir set failed", err)
			}
		}

		if enableUpdateNotification {
			notify.MaybePrintUpdateTextFromGithub()
		}
	},
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	for _, c := range RootCmd.Commands() {
		c.Short = translate.T(c.Short)
		c.Long = translate.T(c.Long)
		c.Flags().VisitAll(func(flag *pflag.Flag) {
			flag.Usage = translate.T(flag.Usage)
		})

		c.SetUsageTemplate(usageTemplate())
	}
	RootCmd.Short = translate.T(RootCmd.Short)
	RootCmd.Long = translate.T(RootCmd.Long)
	RootCmd.Flags().VisitAll(func(flag *pflag.Flag) {
		flag.Usage = translate.T(flag.Usage)
	})
	RootCmd.SetUsageTemplate(usageTemplate())
	if err := RootCmd.Execute(); err != nil {
		// Cobra already outputs the error, typically because the user provided an unknown command.
		os.Exit(exit.BadUsage)
	}
}

// usageTemplate just calls translate.T on the default usage template
// explicitly using the raw string instead of calling c.UsageTemplate()
// so the extractor can find this monstrosity of a string
func usageTemplate() string {
	return fmt.Sprintf(`%s:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

%s:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

%s:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

%s:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

%s:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

%s:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

%s:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

%s{{end}}
`, translate.T("Usage"), translate.T("Aliases"), translate.T("Examples"), translate.T("Available Commands"), translate.T("Flags"), translate.T("Global Flags"), translate.T("Additional help topics"), translate.T(`Use "{{.CommandPath}} [command] --help" for more information about a command.`))
}

// Handle config values for flags used in external packages (e.g. glog)
// by setting them directly, using values from viper when not passed in as args
func setFlagsUsingViper() {
	for _, config := range viperWhiteList {
		var a = pflag.Lookup(config)
		viper.SetDefault(a.Name, a.DefValue)
		// If the flag is set, override viper value
		if a.Changed {
			viper.Set(a.Name, a.Value.String())
		}
		// Viper will give precedence first to calls to the Set command,
		// then to values from the config.yml
		if err := a.Value.Set(viper.GetString(a.Name)); err != nil {
			exit.WithError(fmt.Sprintf("failed to set value for %q", a.Name), err)
		}
		a.Changed = true
	}
}

func init() {
	translate.DetermineLocale()
	RootCmd.PersistentFlags().StringP(config.MachineProfile, "p", constants.DefaultMachineName, `The name of the minikube VM being used. This can be set to allow having multiple instances of minikube independently.`)
	RootCmd.PersistentFlags().StringP(configCmd.Bootstrapper, "b", constants.DefaultClusterBootstrapper, "The name of the cluster bootstrapper that will set up the kubernetes cluster.")

	groups := templates.CommandGroups{
		{
			Message: "Basic Commands:",
			Commands: []*cobra.Command{
				startCmd,
				statusCmd,
				stopCmd,
				deleteCmd,
				dashboardCmd,
			},
		},
		{
			Message: "Images Commands:",
			Commands: []*cobra.Command{
				dockerEnvCmd,
				cacheCmd,
			},
		},
		{
			Message: "Configuration and Management Commands:",
			Commands: []*cobra.Command{
				configCmd.AddonsCmd,
				configCmd.ConfigCmd,
				configCmd.ProfileCmd,
				updateContextCmd,
			},
		},
		{
			Message: "Networking and Connectivity Commands:",
			Commands: []*cobra.Command{
				serviceCmd,
				tunnelCmd,
			},
		},
		{
			Message: "Advanced Commands:",
			Commands: []*cobra.Command{
				mountCmd,
				sshCmd,
				kubectlCmd,
			},
		},
		{
			Message: "Troubleshooting Commands:",
			Commands: []*cobra.Command{
				sshKeyCmd,
				ipCmd,
				logsCmd,
				updateCheckCmd,
				versionCmd,
			},
		},
	}
	groups.Add(RootCmd)

	// any not grouped command will show in Other Commands group.
	RootCmd.AddCommand(completionCmd)
	templates.ActsAsRootCommand(RootCmd, []string{"options"}, groups...)

	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	if err := viper.BindPFlags(RootCmd.PersistentFlags()); err != nil {
		exit.WithError("Unable to bind flags", err)
	}
	cobra.OnInitialize(initConfig)

}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	configPath := constants.ConfigFile
	viper.SetConfigFile(configPath)
	viper.SetConfigType("json")
	err := viper.ReadInConfig()
	if err != nil {
		glog.Warningf("Error reading config file at %s: %v", configPath, err)
	}
	setupViper()
}

func setupViper() {
	viper.SetEnvPrefix(constants.MinikubeEnvPrefix)
	// Replaces '-' in flags with '_' in env variables
	// e.g. iso-url => $ENVPREFIX_ISO_URL
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	viper.SetDefault(config.WantUpdateNotification, true)
	viper.SetDefault(config.ReminderWaitPeriodInHours, 24)
	viper.SetDefault(config.WantReportError, false)
	viper.SetDefault(config.WantReportErrorPrompt, true)
	viper.SetDefault(config.WantKubectlDownloadMsg, true)
	viper.SetDefault(config.WantNoneDriverWarning, true)
	viper.SetDefault(config.ShowDriverDeprecationNotification, true)
	viper.SetDefault(config.ShowBootstrapperDeprecationNotification, true)
	setFlagsUsingViper()
}

// getClusterBootstrapper returns a new bootstrapper for the cluster
func getClusterBootstrapper(api libmachine.API, bootstrapperName string) (bootstrapper.Bootstrapper, error) {
	var b bootstrapper.Bootstrapper
	var err error
	switch bootstrapperName {
	case bootstrapper.BootstrapperTypeKubeadm:
		b, err = kubeadm.NewKubeadmBootstrapper(api)
		if err != nil {
			return nil, errors.Wrap(err, "getting kubeadm bootstrapper")
		}
	default:
		return nil, fmt.Errorf("unknown bootstrapper: %s", bootstrapperName)
	}

	return b, nil
}
