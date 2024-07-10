// Copyright 2019-2024 The Inspektor Gadget authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tests

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	gadgettesting "github.com/inspektor-gadget/inspektor-gadget/gadgets/testing"
	igtesting "github.com/inspektor-gadget/inspektor-gadget/pkg/testing"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/testing/containers"
	igrunner "github.com/inspektor-gadget/inspektor-gadget/pkg/testing/ig"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/testing/match"
	"github.com/inspektor-gadget/inspektor-gadget/pkg/testing/utils"
	eventtypes "github.com/inspektor-gadget/inspektor-gadget/pkg/types"
)

type traceBindEvent struct {
	eventtypes.CommonData

	Addr       utils.L4Endpoint `json:"addr"`
	Timestamp  string           `json:"timestamp"`
	MountNsID  uint64           `json:"mount_ns_id"`
	Pid        uint32           `json:"pid"`
	Uid        uint32           `json:"uid"`
	Gid        uint32           `json:"gid"`
	Ret        int32            `json:"ret"`
	Opts       string           `json:"opts"`
	Comm       string           `json:"comm"`
	BoundDevIF uint32           `json:"bound_dev_if"`
}

func TestTraceBind(t *testing.T) {
	gadgettesting.RequireEnvironmentVariables(t)
	utils.InitTest(t)

	containerFactory, err := containers.NewContainerFactory(utils.Runtime)
	require.NoError(t, err, "new container factory")
	containerName := "test-trace-bind"
	containerImage := "ghcr.io/inspektor-gadget/ci/busybox:latest"

	var ns string
	containerOpts := []containers.ContainerOption{containers.WithContainerImage(containerImage)}

	if utils.CurrentTestComponent == utils.KubectlGadgetTestComponent {
		ns = utils.GenerateTestNamespaceName(t, "test-trace-bind")
		containerOpts = append(containerOpts, containers.WithContainerNamespace(ns))
	}

	testContainer := containerFactory.NewContainer(
		containerName,
		"while true; do setuidgid 1000:1111 nc -l -s 127.0.0.1 -p 9090 -w 1; sleep 0.1; done",
		containerOpts...,
	)

	testContainer.Start(t)
	t.Cleanup(func() {
		testContainer.Stop(t)
	})

	var runnerOpts []igrunner.Option
	var testingOpts []igtesting.Option
	commonDataOpts := []utils.CommonDataOption{utils.WithContainerImageName(containerImage), utils.WithContainerID(testContainer.ID())}

	switch utils.CurrentTestComponent {
	case utils.IgLocalTestComponent:
		runnerOpts = append(runnerOpts, igrunner.WithFlags(fmt.Sprintf("-r=%s", utils.Runtime), "--timeout=5"))
	case utils.KubectlGadgetTestComponent:
		runnerOpts = append(runnerOpts, igrunner.WithFlags(fmt.Sprintf("-n=%s", ns), "--timeout=5"))
		testingOpts = append(testingOpts, igtesting.WithCbBeforeCleanup(utils.PrintLogsFn(ns)))
		commonDataOpts = append(commonDataOpts, utils.WithK8sNamespace(ns))
	}

	runnerOpts = append(runnerOpts, igrunner.WithValidateOutput(
		func(t *testing.T, output string) {
			expectedEntry := &traceBindEvent{
				CommonData: utils.BuildCommonData(containerName, commonDataOpts...),
				Addr: utils.L4Endpoint{
					Addr:    "127.0.0.1",
					Version: 4,
					Port:    9090,
					Proto:   6,
				},
				Comm: "nc",
				Opts: "REUSEADDRESS",
				Uid:  1000,
				Gid:  1111,

				// Check the existence of the following fields
				Timestamp: utils.NormalizedStr,
				Pid:       utils.NormalizedInt,
				MountNsID: utils.NormalizedInt,
			}

			normalize := func(e *traceBindEvent) {
				utils.NormalizeCommonData(&e.CommonData)
				utils.NormalizeString(&e.Timestamp)
				utils.NormalizeInt(&e.MountNsID)
				utils.NormalizeInt(&e.Pid)
			}

			match.MatchEntries(t, match.JSONMultiObjectMode, output, normalize, expectedEntry)
		},
	))

	traceBindCmd := igrunner.New("trace_bind", runnerOpts...)

	igtesting.RunTestSteps([]igtesting.TestStep{traceBindCmd}, t, testingOpts...)
}
