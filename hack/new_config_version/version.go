/*
Copyright 2019 The Skaffold Authors

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

package main

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/go-github/github"

	"github.com/sirupsen/logrus"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/latest"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema"
)

// Before: prev -> current (latest)
// After:  prev -> current -> new (latest)
func main() {
	logrus.SetLevel(logrus.DebugLevel)
	new := os.Args[1]
	current := latest.Version
	fmt.Printf("current version: %s", current)
	prev := strings.TrimPrefix(schema.SchemaVersions[len(schema.SchemaVersions)-2].APIVersion, "skaffold/")
	logrus.Infof("previous version: %s", prev)

	logrus.Infof("checking for released status of %s...", prev)
	client := github.NewClient(nil)

	releases, _, _ := client.Repositories.ListReleases(context.Background(), "GoogleContainerTools", "skaffold", &github.ListOptions{})
	lastTag := *releases[0].TagName

	logrus.Infof("last release tag: %s", lastTag)

	configURL := fmt.Sprintf("https://raw.githubusercontent.com/GoogleContainerTools/skaffold/%s/pkg/skaffold/schema/latest/config.go", lastTag)
	resp, err := http.Get(configURL)
	if err != nil {
		logrus.Fatalf("can't determine latest released config lastReleasedVersion, failed to download %s: %s", configURL, err)
	}

	config, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Fatalf("failed to read during download %s, err: %s", configURL, err)
	}
	versionPattern := regexp.MustCompile("const Version string = \"(skaffold/.*)\"")
	lastReleasedVersion := versionPattern.FindStringSubmatch(string(config))[1]

	logrus.Infof("last released version: %s", lastReleasedVersion)

	if lastReleasedVersion != current {
		logrus.Fatalf("There is no need to create a new version, %s is still not released", current)
	}

	// Create a package for current version
	walk(path("latest"), func(file string, info os.FileInfo) {
		if !info.IsDir() {
			cp(file, path(current, info.Name()))
			sed(path(current, info.Name()), "package latest", "package "+current)
		}
	})

	// Create code to upgrade from current to new
	cp(path(prev, "upgrade.go"), path(current, "upgrade.go"))
	sed(path(current, "upgrade.go"), current, new)
	sed(path(current, "upgrade.go"), prev, current)

	// Create a test for the upgrade from current to new
	cp(path(prev, "upgrade_test.go"), path(current, "upgrade_test.go"))
	sed(path(current, "upgrade_test.go"), current, new)
	sed(path(current, "upgrade_test.go"), prev, current)

	// Previous version now upgrades to current instead of latest
	sed(path(prev, "upgrade.go"), "latest", current)
	sed(path(prev, "upgrade_test.go"), "latest", current)

	// Latest uses the new version
	sed(path("latest", "config.go"), current, new)

	// Update skaffold.yaml in integration tests
	walk("integration", func(path string, info os.FileInfo) {
		if info.Name() == "skaffold.yaml" {
			sed(path, current, new)
		}
	})

	// Add the new version to the list of versions
	lines := lines(path("versions.go"))
	var content string
	for _, line := range lines {
		content += line + "\n"
		if strings.Contains(line, prev) {
			content += strings.ReplaceAll(line, prev, current) + "\n"
		}
	}
	write(path("versions.go"), []byte(content))

	// Update the docs with the new version
	sed("docs/config.toml", current, new)
}

func path(elem ...string) string {
	return filepath.Join(append([]string{"pkg", "skaffold", "schema"}, elem...)...)
}

func read(path string) []byte {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		panic("unable to read " + path)
	}
	return buf
}

func write(path string, buf []byte) {
	if err := ioutil.WriteFile(path, buf, os.ModePerm); err != nil {
		panic("unable to write " + path)
	}
}

func sed(path string, old, new string) {
	buf := read(path)
	replaced := regexp.MustCompile(old).ReplaceAll(buf, []byte(new))
	write(path, replaced)
}

func cp(path string, dest string) {
	buf := read(path)
	os.Mkdir(filepath.Dir(dest), os.ModePerm)
	write(dest, buf)
}

func lines(path string) []string {
	file, err := os.Open(path)
	if err != nil {
		panic("unable to open " + path)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func walk(root string, fn func(path string, info os.FileInfo)) {
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		fn(path, info)
		return nil
	}); err != nil {
		panic("unable to list files")
	}
}
