// Copyright 2019-2021 The Inspektor Gadget authors
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

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kinvolk/inspektor-gadget/cmd/kubectl-gadget/utils"
	dnstypes "github.com/kinvolk/inspektor-gadget/pkg/gadgets/dns/types"
)

const (
	FMT_ALL   = "%-16.16s %-16.16s %-30.30s %-9.9s %s"
	FMT_SHORT = "%-30.30s %-9.9s %s"
)

var colLens = map[string]int{
	"pkt_type": 10,
	"name":     30,
}

var dnsCmd = &cobra.Command{
	Use:   "dns",
	Short: "Trace DNS requests",
	Run: func(cmd *cobra.Command, args []string) {
		transform := transformLine

		switch {
		case params.OutputMode == utils.OutputModeJson: // don't print any header
		case params.OutputMode == utils.OutputModeCustomColumns:
			table := utils.NewTableFormater(params.CustomColumns, colLens)
			fmt.Println(table.GetHeader())
			transform = table.GetTransformFunc()
		case params.AllNamespaces:
			fmt.Printf(FMT_ALL+"\n",
				"NODE",
				"NAMESPACE",
				"POD",
				"TYPE",
				"NAME",
			)
		default:
			fmt.Printf(FMT_SHORT+"\n",
				"POD",
				"TYPE",
				"NAME",
			)
		}

		config := &utils.TraceConfig{
			GadgetName:       "dns",
			Operation:        "start",
			TraceOutputMode:  "Stream",
			TraceOutputState: "Started",
			CommonFlags:      &params,
		}

		err := utils.RunTraceAndPrintStream(config, transform)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)

			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(dnsCmd)
	utils.AddCommonFlags(dnsCmd, &params)
}

func transformLine(line string) string {
	event := &dnstypes.Event{}
	json.Unmarshal([]byte(line), event)

	podMsgSuffix := ""
	if event.Namespace != "" && event.Pod != "" {
		podMsgSuffix = ", pod " + event.Namespace + "/" + event.Pod
	}

	if event.Err != "" {
		return fmt.Sprintf("Error on node %s%s: %s: %s", event.Node, podMsgSuffix, event.Notice, event.Err)
	}
	if event.Notice != "" {
		if !params.Verbose {
			return ""
		}
		return fmt.Sprintf("Notice on node %s%s: %s", event.Node, podMsgSuffix, event.Notice)
	}
	if params.AllNamespaces {
		return fmt.Sprintf(FMT_ALL, event.Node, event.Namespace, event.Pod, event.PktType, event.DNSName)
	} else {
		return fmt.Sprintf(FMT_SHORT, event.Pod, event.PktType, event.DNSName)
	}
}
