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

package gadgets

import (
	gadgetv1alpha1 "github.com/kinvolk/inspektor-gadget/pkg/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

type Gadget interface {
	Delete(trace types.NamespacedName) error
	Operation(trace *gadgetv1alpha1.Trace, operation string, params map[string]string) StatusUpdate
}

type StatusUpdate struct {
	Output         *string
	State          *string
	OperationError *string
}
