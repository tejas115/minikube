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
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	cmdUtil "k8s.io/minikube/cmd/util"
	"k8s.io/minikube/pkg/minikube/cluster"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/pkg/minikube/machine"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stops a running local kubernetes cluster.",
	Long: `Stops a local kubernetes cluster running in Virtualbox. This command stops the VM
itself, leaving all files intact. The cluster can be started again with the "start" command.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Stopping local Kubernetes cluster...")
		api, err := machine.NewAPIClient(clientType)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting client: %s\n", err)
			os.Exit(1)
		}
		defer api.Close()

		if err = cluster.StopHost(api); err != nil {
			fmt.Println("Error stopping machine: ", err)
			cmdUtil.MaybeReportErrorAndExit(err)
		}
		fmt.Println("Machine stopped.")

		gob.Register(SX{})
		gob.Register(os.Process{})
		encoded, err := ioutil.ReadFile(filepath.Join(constants.GetMinipath(), ".mount-process"))
		if err != nil {
			fmt.Println("Error stopping mount daemon: ", err)
			cmdUtil.MaybeReportErrorAndExit(err)
		}
		decoded := FromGOB64(string(encoded))
		original, ok := decoded["mountProcess"].(os.Process)
		if !ok {
			fmt.Println("Error decoding mount process for deletion")
		}
		original.Kill()
	},
}

func init() {
	RootCmd.AddCommand(stopCmd)
}
