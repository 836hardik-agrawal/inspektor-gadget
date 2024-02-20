// Copyright 2024 The Inspektor Gadget authors
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

// Package btfhelpers provides a couple of helper functions to bridge Go's reflection system with
// types from BTF
package btfhelpers

import (
	"reflect"

	"github.com/cilium/ebpf/btf"
)

func GetType(typ btf.Type) (reflect.Type, []string) {
	switch typedMember := typ.(type) {
	case *btf.Array:
		arrType, typeNames := getSimpleType(typedMember.Type)
		if arrType == nil {
			return nil, nil
		}
		return reflect.ArrayOf(int(typedMember.Nelems), arrType), append(typeNames, typ.TypeName())
	default:
		arrType, typeNames := getSimpleType(typ)
		return arrType, append(typeNames, typ.TypeName())
	}
}

func GetUnderlyingType(tf *btf.Typedef) (btf.Type, []string, error) {
	switch typedMember := tf.Type.(type) {
	case *btf.Typedef:
		typ, typeNames, err := GetUnderlyingType(typedMember)
		return typ, append(typeNames, tf.TypeName()), err
	default:
		return typedMember, []string{tf.TypeName()}, nil
	}
}

func getSimpleType(typ btf.Type) (reflect.Type, []string) {
	switch typedMember := typ.(type) {
	case *btf.Int:
		switch typedMember.Encoding {
		case btf.Signed:
			switch typedMember.Size {
			case 1:
				return reflect.TypeOf(int8(0)), []string{typ.TypeName()}
			case 2:
				return reflect.TypeOf(int16(0)), []string{typ.TypeName()}
			case 4:
				return reflect.TypeOf(int32(0)), []string{typ.TypeName()}
			case 8:
				return reflect.TypeOf(int64(0)), []string{typ.TypeName()}
			}
		case btf.Unsigned:
			switch typedMember.Size {
			case 1:
				return reflect.TypeOf(uint8(0)), []string{typ.TypeName()}
			case 2:
				return reflect.TypeOf(uint16(0)), []string{typ.TypeName()}
			case 4:
				return reflect.TypeOf(uint32(0)), []string{typ.TypeName()}
			case 8:
				return reflect.TypeOf(uint64(0)), []string{typ.TypeName()}
			}
		case btf.Bool:
			return reflect.TypeOf(false), []string{typ.TypeName()}
		case btf.Char:
			return reflect.TypeOf(uint8(0)), []string{typ.TypeName()}
		}
	case *btf.Float:
		switch typedMember.Size {
		case 4:
			return reflect.TypeOf(float32(0)), []string{typ.TypeName()}
		case 8:
			return reflect.TypeOf(float64(0)), []string{typ.TypeName()}
		}
	case *btf.Typedef:
		typ, typeNames, _ := GetUnderlyingType(typedMember)
		refType, typeNames2 := getSimpleType(typ)
		return refType, append(append(typeNames, typ.TypeName()), typeNames2...)
	case *btf.Array: // TODO: this has to be supported here
		arrType, typeNames := getSimpleType(typedMember.Type)
		if arrType == nil {
			return nil, nil
		}
		return reflect.ArrayOf(int(typedMember.Nelems), arrType), append(typeNames, typ.TypeName())
	}
	return nil, nil
}
