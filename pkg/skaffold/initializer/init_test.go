/*
Copyright 2020 The Skaffold Authors

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

package initializer

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/config"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema"
	"github.com/GoogleContainerTools/skaffold/testutil"
)

func TestDoInit(t *testing.T) {
	tests := []struct {
		name      string
		dir       string
		config    Config
		shouldErr bool
	}{
		//TODO: mocked kompose test
		{
			name: "getting-started",
			dir:  "testdata/init/hello",
			config: Config{
				Opts: config.SkaffoldOptions{
					ConfigurationFile: "skaffold.yaml.out",
				},
			},
		},
		{
			name: "ignore existing tags",
			dir:  "testdata/init/ignore-tags",
			config: Config{
				Opts: config.SkaffoldOptions{
					ConfigurationFile: "skaffold.yaml.out",
				},
			},
		},
		{
			name: "microservices (backwards compatibility)",
			dir:  "testdata/init/microservices",
			config: Config{
				CliArtifacts: []string{
					"leeroy-app/Dockerfile=gcr.io/k8s-skaffold/leeroy-app",
					"leeroy-web/Dockerfile=gcr.io/k8s-skaffold/leeroy-web",
				},
				Opts: config.SkaffoldOptions{
					ConfigurationFile: "skaffold.yaml.out",
				},
			},
		},
		{
			name: "microservices",
			dir:  "testdata/init/microservices",
			config: Config{
				CliArtifacts: []string{
					`{"builder":"Docker","payload":{"path":"leeroy-app/Dockerfile"},"image":"gcr.io/k8s-skaffold/leeroy-app"}`,
					`{"builder":"Docker","payload":{"path":"leeroy-web/Dockerfile"},"image":"gcr.io/k8s-skaffold/leeroy-web"}`,
				},
				Opts: config.SkaffoldOptions{
					ConfigurationFile: "skaffold.yaml.out",
				},
			},
		},
		{
			name: "error writing config file",
			dir:  "testdata/init/microservices",

			config: Config{
				CliArtifacts: []string{
					`{"builder":"Docker","payload":{"path":"leeroy-app/Dockerfile"},"image":"gcr.io/k8s-skaffold/leeroy-app"}`,
					`{"builder":"Docker","payload":{"path":"leeroy-web/Dockerfile"},"image":"gcr.io/k8s-skaffold/leeroy-web"}`,
				},
				Opts: config.SkaffoldOptions{
					// erroneous config file as . is a directory
					ConfigurationFile: ".",
				},
			},
			shouldErr: true,
		},
	}
	for _, test := range tests {
		testutil.Run(t, test.name, func(t *testutil.T) {
			t.Chdir(test.dir)
			// we still need as a "no-prompt" mode
			test.config.Force = true
			err := DoInit(context.TODO(), os.Stdout, test.config)
			t.CheckError(test.shouldErr, err)
			checkGeneratedConfig(t, ".")
		})
	}
}

func TestDoInitAnalyze(t *testing.T) {
	tests := []struct {
		name        string
		dir         string
		config      Config
		expectedOut string
		shouldErr   bool
	}{
		{
			name: "analyze microservices",
			dir:  "testdata/init/microservices",
			config: Config{
				Analyze: true,
			},
			expectedOut: strip(`{
							"dockerfiles":["leeroy-app/Dockerfile","leeroy-web/Dockerfile"],
							"images":["gcr.io/k8s-skaffold/leeroy-app","gcr.io/k8s-skaffold/leeroy-web"]
							}`) + "\n",
		},
		{
			name: "analyze microservices new format",
			dir:  "testdata/init/microservices",
			config: Config{
				Analyze:       true,
				EnableJibInit: true,
			},
			expectedOut: strip(`{
									"builders":[
										{"name":"Docker","payload":{"path":"leeroy-app/Dockerfile"}},
										{"name":"Docker","payload":{"path":"leeroy-web/Dockerfile"}}
									],
									"images":[
										{"name":"gcr.io/k8s-skaffold/leeroy-app","foundMatch":false},
										{"name":"gcr.io/k8s-skaffold/leeroy-web","foundMatch":false}]}`) + "\n",
		},
	}

	for _, test := range tests {
		testutil.Run(t, test.name, func(t *testutil.T) {
			var out bytes.Buffer
			t.Chdir(test.dir)
			err := DoInit(context.TODO(), &out, test.config)
			t.CheckErrorAndDeepEqual(test.shouldErr, err, test.expectedOut, out.String())
		})
	}
}

func strip(s string) string {
	cutString := "\n\t\r"
	stripped := ""
	for _, r := range s {
		if strings.ContainsRune(cutString, r) {
			continue
		}
		stripped = fmt.Sprintf("%s%c", stripped, r)
	}
	return stripped
}

func checkGeneratedConfig(t *testutil.T, dir string) {
	expectedOutput, err := schema.ParseConfig(filepath.Join(dir, "skaffold.yaml"), false)
	t.CheckNoError(err)

	output, err := schema.ParseConfig(filepath.Join(dir, "skaffold.yaml.out"), false)
	t.CheckNoError(err)
	t.CheckDeepEqual(expectedOutput, output)
}
