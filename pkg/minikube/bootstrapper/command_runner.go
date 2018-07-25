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

package bootstrapper

import (
	"fmt"
	"io"
	"path/filepath"

	"k8s.io/minikube/pkg/minikube/assets"
)

// CommandRunner represents an interface to run commands.
type CommandRunner interface {
	// Run starts the specified command in a bash shell and waits for it to complete.
	Run(cmd string) error

	// RunWithOutputTo runs the command (as in Run()) and redirects both its
	// stdout and stderr to `out`. A typical usage is:
	//
	//          var b bytes.Buffer
	//          RunWithOutputTo(cmd, &b)
	//          fmt.Println(b.Bytes())
	//
	// Or, you can set out to os.Stdout, the command output and
	// error would show on your terminal immediately before you
	// cmd exit. This is useful for a long run command such as
	// continuously print running logs.
	RunWithOutputTo(cmd string, out io.Writer) error

	// RunWithOutput runs the command (as in Run()) and returns its combined standard
	// output and standard error.
	RunWithOutput(cmd string) (string, error)

	// Copy is a convenience method that runs a command to copy a file
	Copy(assets.CopyableFile) error

	// Remove is a convenience method that runs a command to remove a file
	Remove(assets.CopyableFile) error
}

func getDeleteFileCommand(f assets.CopyableFile) string {
	return fmt.Sprintf("sudo rm %s", filepath.Join(f.GetTargetDir(), f.GetTargetName()))
}
