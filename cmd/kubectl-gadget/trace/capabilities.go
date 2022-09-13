// Copyright 2022 The Inspektor Gadget authors
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
	"fmt"
	"strconv"
	"strings"

	commontrace "github.com/kinvolk/inspektor-gadget/cmd/common/trace"
	commonutils "github.com/kinvolk/inspektor-gadget/cmd/common/utils"
	"github.com/kinvolk/inspektor-gadget/cmd/kubectl-gadget/utils"
	"github.com/kinvolk/inspektor-gadget/pkg/gadgets/trace/capabilities/types"

	"github.com/spf13/cobra"
)

type CapabilitiesParser struct {
	commonutils.BaseParser[types.Event]
}

func newCapabilitiesCmd() *cobra.Command {
	commonFlags := &utils.CommonFlags{
		OutputConfig: commonutils.OutputConfig{
			// The columns that will be used in case the user does not specify
			// which specific columns they want to print.
			CustomColumns: []string{
				"node",
				"namespace",
				"pod",
				"container",
				"pid",
				"comm",
				"uid",
				"cap",
				"name",
				"audit",
				"verdict",
			},
		},
	}

	// flags
	var auditOnly bool

	cmd := &cobra.Command{
		Use:   "capabilities",
		Short: "Trace security capability checks",
		RunE: func(cmd *cobra.Command, args []string) error {
			capabilitiesGadget := &TraceGadget[types.Event]{
				name:        "capabilities",
				commonFlags: commonFlags,
				parser:      NewCapabilitiesParser(&commonFlags.OutputConfig),
				params: map[string]string{
					types.AuditOnlyParam: strconv.FormatBool(auditOnly),
				},
			}

			return capabilitiesGadget.Run()
		},
	}

	cmd.PersistentFlags().BoolVarP(
		&auditOnly,
		"audit-only",
		"",
		true,
		"Only show audit checks",
	)

	utils.AddCommonFlags(cmd, commonFlags)

	return cmd
}

func NewCapabilitiesParser(outputConfig *commonutils.OutputConfig) commontrace.TraceParser[types.Event] {
	columnsWidth := map[string]int{
		"node":      -16,
		"namespace": -16,
		"pod":       -30,
		"container": -16,
		"pid":       -7,
		"uid":       -7,
		"comm":      -16,
		"cap":       -4,
		"name":      -16,
		"audit":     -6,
		"verdict":   -6,
	}

	return &CapabilitiesParser{
		BaseParser: commonutils.NewBaseWidthParser[types.Event](columnsWidth, outputConfig),
	}
}

func (p *CapabilitiesParser) TransformToColumns(event *types.Event) string {
	var sb strings.Builder

	for _, col := range p.OutputConfig.CustomColumns {
		switch col {
		case "node":
			sb.WriteString(fmt.Sprintf("%*s", p.ColumnsWidth[col], event.Node))
		case "namespace":
			sb.WriteString(fmt.Sprintf("%*s", p.ColumnsWidth[col], event.Namespace))
		case "pod":
			sb.WriteString(fmt.Sprintf("%*s", p.ColumnsWidth[col], event.Pod))
		case "container":
			sb.WriteString(fmt.Sprintf("%*s", p.ColumnsWidth[col], event.Container))
		case "pid":
			sb.WriteString(fmt.Sprintf("%*d", p.ColumnsWidth[col], event.Pid))
		case "uid":
			sb.WriteString(fmt.Sprintf("%*d", p.ColumnsWidth[col], event.UID))
		case "comm":
			sb.WriteString(fmt.Sprintf("%*s", p.ColumnsWidth[col], event.Comm))
		case "cap":
			sb.WriteString(fmt.Sprintf("%*d", p.ColumnsWidth[col], event.Cap))
		case "name":
			sb.WriteString(fmt.Sprintf("%*s", p.ColumnsWidth[col], event.CapName))
		case "audit":
			sb.WriteString(fmt.Sprintf("%*d", p.ColumnsWidth[col], event.Audit))
		case "verdict":
			sb.WriteString(fmt.Sprintf("%*s", p.ColumnsWidth[col], event.Verdict))
		default:
			continue
		}

		// Needed when field is larger than the predefined columnsWidth.
		sb.WriteRune(' ')
	}

	return sb.String()
}
