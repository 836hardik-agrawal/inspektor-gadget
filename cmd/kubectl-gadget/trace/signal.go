// Copyright 2019-2022 The Inspektor Gadget authors
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

package trace

import (
	"strconv"

	commontrace "github.com/kinvolk/inspektor-gadget/cmd/common/trace"
	commonutils "github.com/kinvolk/inspektor-gadget/cmd/common/utils"
	"github.com/kinvolk/inspektor-gadget/cmd/kubectl-gadget/utils"
	signalTypes "github.com/kinvolk/inspektor-gadget/pkg/gadgets/trace/signal/types"

	"github.com/spf13/cobra"
)

func newSignalCmd() *cobra.Command {
	var commonFlags utils.CommonFlags
	var flags commontrace.SignalFlags

	runCmd := func(cmd *cobra.Command, args []string) error {
		filters, _ := cmd.PersistentFlags().GetStringArray("filter")

		signalGadget := &TraceGadget[signalTypes.Event]{
			name:        "sigsnoop",
			commonFlags: &commonFlags,
			parser:      commontrace.NewParserWithK8sInfo(&commonFlags.OutputConfig, signalTypes.MustGetColumns(), commonutils.WithFilters(filters)),
			params: map[string]string{
				"signal": flags.Sig,
				"pid":    strconv.FormatUint(flags.Pid, 10),
				"failed": strconv.FormatBool(flags.Failed),
			},
		}

		return signalGadget.Run()
	}

	cmd := commontrace.NewSignalCmd(runCmd, &flags)

	utils.AddCommonFlags(cmd, &commonFlags)

	return cmd
}
