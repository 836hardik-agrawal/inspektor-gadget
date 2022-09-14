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
	commonutils "github.com/kinvolk/inspektor-gadget/cmd/common/utils"
	"github.com/kinvolk/inspektor-gadget/pkg/columns"
	eventtypes "github.com/kinvolk/inspektor-gadget/pkg/types"

	"github.com/spf13/cobra"
)

type TraceEvent interface {
	any

	// The Go compiler does not support accessing a struct field x.f where x is
	// of type parameter type even if all types in the type parameter's type set
	// have a field f. We may remove this restriction in Go 1.19. See
	// https://tip.golang.org/doc/go1.18#generics.
	GetBaseEvent() eventtypes.Event
}

// TraceParser defines the interface that every trace-gadget parser has to
// implement.
type TraceParser[Event TraceEvent] interface {
	// TransformToColumns is called to transform an event to columns.
	TransformToColumns(event *Event) string

	// BuildColumnsHeader returns a header with the requested custom columns
	// that exist in the predefined columns list. The columns are separated by
	// the predefined width.
	BuildColumnsHeader() string
}

func NewCommonTraceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "trace",
		Short: "Trace and print system events",
	}
}

func NewParser[T TraceEvent](outputConfig *commonutils.OutputConfig, columns *columns.Columns[T], opts ...commonutils.Option) TraceParser[T] {
	return commonutils.NewGadgetParser[T](outputConfig, columns, opts...)
}

func NewParserWithK8sInfo[T TraceEvent](outputConfig *commonutils.OutputConfig, columns *columns.Columns[T], opts ...commonutils.Option) TraceParser[T] {
	return commonutils.NewGadgetParser[T](outputConfig, columns, append([]commonutils.Option{commonutils.WithMetadataTag(commonutils.KubernetesTag)}, opts...)...)
}

func NewParserWithRuntimeInfo[T TraceEvent](outputConfig *commonutils.OutputConfig, columns *columns.Columns[T], opts ...commonutils.Option) TraceParser[T] {
	return commonutils.NewGadgetParser[T](outputConfig, columns, append([]commonutils.Option{commonutils.WithMetadataTag(commonutils.ContainerRuntimeTag)}, opts...)...)
}
