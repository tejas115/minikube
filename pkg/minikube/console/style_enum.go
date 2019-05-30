/*
Copyright 2019 The Kubernetes Authors All rights reserved.

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

package console

type StyleEnum int
const (
	Happy StyleEnum = iota
	SuccessType
	FailureType
	Conflict
	FatalType
	Notice
	Ready
	Running
	Provisioning
	Restarting
	Reconfiguring
	Stopping
	Stopped
	WarningType
	Waiting
	WaitingPods
	Usage
	Launch
	Sad
	ThumbsUp
	Option
	Command
	LogEntry
	Crushed
	Url
	Documentation
	Issues
	Issue
	Check
	IsoDownload
	FileDownload
	Caching
	StartingVm
	StartingNone
	Resetting
	DeletingHost
	Copying
	Connectivity
	Internet
	Mounting
	Celebrate
	ContainerRuntime
	Docker
	Crio
	Containerd
	Permissions
	Enabling
	Shutdown
	Pulling
	Verifying
	VerifyingNoLine
	Kubectl
	Meh
	Embarrassed
	Tip
	Unmount
	MountOptions
	Fileserver
)

func (style StyleEnum) String() string {
	var s = []string{"Happy",
		"SuccessType",
		"FailureType",
		"Conflict",
		"FatalType",
		"Notice",
		"Ready",
		"Running",
		"Provisioning",
		"Restarting",
		"Reconfiguring",
		"Stopping",
		"Stopped",
		"WarningType",
		"Waiting",
		"WaitingPods",
		"Usage",
		"Launch",
		"Sad",
		"ThumbsUp",
		"Option",
		"Command",
		"LogEntry",
		"Crushed",
		"Url",
		"Documentation",
		"Issues",
		"Issue",
		"Check",
		"IsoDownload",
		"FileDownload",
		"Caching",
		"StartingVm",
		"StartingNone",
		"Resetting",
		"DeletingHost",
		"Copying",
		"Connectivity",
		"Internet",
		"Mounting",
		"Celebrate",
		"ContainerRuntime",
		"Docker",
		"Crio", 
		"Containerd",
		"Permissions",
		"Enabling",
		"Shutdown",
		"Pulling",
		"Verifying",
		"VerifyingNoLine",
		"Kubectl",
		"Meh",  
		"Embarrassed",
		"Tip",  
		"Unmount",
		"MountOptions",
		"Fileserver"}

	if style > Fileserver{
		return "Unknown"
	}

	return s[style]
}