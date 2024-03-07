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

package util

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/blang/semver/v4"
	units "github.com/docker/go-units"
	"github.com/pkg/errors"
)

const (
	downloadURL = "https://storage.googleapis.com/minikube/releases/%s/minikube-%s-%s%s"
)

// CalculateSizeInMB returns the number of MB in the human readable string
func CalculateSizeInMB(humanReadableSize string) (int, error) {
	_, err := strconv.ParseInt(humanReadableSize, 10, 64)
	if err == nil {
		humanReadableSize += "mb"
	}
	// parse the size suffix binary instead of decimal so that 1G -> 1024MB instead of 1000MB
	size, err := units.RAMInBytes(humanReadableSize)
	if err != nil {
		return 0, fmt.Errorf("FromHumanSize: %v", err)
	}

	return int(size / units.MiB), nil
}

// ConvertMBToBytes converts MB to bytes
func ConvertMBToBytes(mbSize int) int64 {
	return int64(mbSize) * units.MiB
}

// ConvertBytesToMB converts bytes to MB
func ConvertBytesToMB(byteSize int64) int {
	return int(ConvertUnsignedBytesToMB(uint64(byteSize)))
}

// ConvertUnsignedBytesToMB converts bytes to MB
func ConvertUnsignedBytesToMB(byteSize uint64) int64 {
	return int64(byteSize / units.MiB)
}

// GetBinaryDownloadURL returns a suitable URL for the platform and arch
func GetBinaryDownloadURL(version, platform, arch string) string {
	switch platform {
	case "windows":
		return fmt.Sprintf(downloadURL, version, platform, arch, ".exe")
	default:
		return fmt.Sprintf(downloadURL, version, platform, arch, "")
	}
}

// ChownR does a recursive os.Chown
func ChownR(path string, uid, gid int) error {
	return filepath.Walk(path, func(name string, _ os.FileInfo, err error) error {
		if err == nil {
			err = os.Chown(name, uid, gid)
		}
		return err
	})
}

// MaybeChownDirRecursiveToMinikubeUser changes ownership of a dir, if requested
func MaybeChownDirRecursiveToMinikubeUser(dir string) error {
	if os.Getenv("CHANGE_MINIKUBE_NONE_USER") != "" && os.Getenv("SUDO_USER") != "" {
		username := os.Getenv("SUDO_USER")
		usr, err := user.Lookup(username)
		if err != nil {
			return errors.Wrap(err, "Error looking up user")
		}
		uid, err := strconv.Atoi(usr.Uid)
		if err != nil {
			return errors.Wrapf(err, "Error parsing uid for user: %s", username)
		}
		gid, err := strconv.Atoi(usr.Gid)
		if err != nil {
			return errors.Wrapf(err, "Error parsing gid for user: %s", username)
		}
		if err := ChownR(dir, uid, gid); err != nil {
			return errors.Wrapf(err, "Error changing ownership for: %s", dir)
		}
	}
	return nil
}

// ParseKubernetesVersion parses the Kubernetes version
func ParseKubernetesVersion(version string) (semver.Version, error) {
	return semver.Make(version[1:])
}

// RemoveDuplicateStrings takes a string slice and returns a slice without duplicates
func RemoveDuplicateStrings(initial []string) []string {
	var result []string
	m := make(map[string]bool, len(initial))
	for _, v := range initial {
		if _, ok := m[v]; ok {
			continue
		}
		m[v] = true
		result = append(result, v)
	}
	return result
}

// remove the specified line from the given file
func RemoveLineFromFile(knownHostLine string, filePath string) error {
	fd, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("failed to open %s: %v", filePath, err)
	}
	defer fd.Close()

	// read each line from known_hosts and find theline we want to delete
	scanner := bufio.NewScanner(fd)
	newLines := make([]string, 0)
	for scanner.Scan() {
		line := string(scanner.Bytes())
		if strings.TrimSpace(line) != strings.TrimSpace(knownHostLine) {
			// remove the line which is identical with the key
			newLines = append(newLines, line)
		}
	}
	// remove the contents and move to the head of this file
	if err := fd.Truncate(0); err != nil {
		return fmt.Errorf("failed to write to %s: %v", filePath, err)
	}
	if _, err := fd.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to write to %s: %v", filePath, err)
	}

	// write the new content into the file

	for _, line := range newLines {
		if _, err := fmt.Fprintln(fd, line); err != nil {
			return fmt.Errorf("failed to write to %s: %v", filePath, err)
		}
	}
	return nil
}
