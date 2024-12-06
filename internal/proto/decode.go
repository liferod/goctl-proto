package proto

import (
	"fmt"
	"github.com/zeromicro/go-zero/tools/goctl/api/spec"
	"strings"
)

var (
	emptyMessage     = Message{"Empty", nil, nil}
	fieldTypeNameMap = func() map[string]string {
		basic := map[string]string{
			"float64":     "double",
			"float32":     "float",
			"int":         "int32",
			"int8":        "int32",
			"int16":       "int32",
			"int32":       "int32",
			"int64":       "int64",
			"uint":        "uint32",
			"uint8":       "uint32",
			"uint16":      "uint32",
			"uint32":      "uint32",
			"uint64":      "uint64",
			"bool":        "bool",
			"string":      "string",
			"byte":        "uint32",
			"*byte":       "uint32",
			"[]byte":      "bytes",
			"*[]byte":     "bytes",
			"[]*byte":     "bytes",
			"*[]*byte":    "bytes",
			"any":         "bytes", // todo: use google.protobuf.Any
			"interface{}": "bytes",
		}
		result := make(map[string]string)
		for goType, protoType := range basic {
			result[goType] = protoType
			if strings.Contains(goType, "byte") {
				continue
			}
			for _, prefix := range []string{
				"*", "[]", "*[]", "[]*", "*[]*",
			} {
				result[prefix+goType] = protoType
			}
		}
		return result
	}()
	mapKeyTypeNameMap = func() map[string]string {
		basic := map[string]string{
			"int":    "int32",
			"int8":   "int32",
			"int16":  "int32",
			"int32":  "int32",
			"int64":  "int64",
			"uint":   "uint32",
			"uint8":  "uint32",
			"uint16": "uint32",
			"uint32": "uint32",
			"uint64": "uint64",
			"string": "string",
			"byte":   "uint32",
		}
		result := make(map[string]string)
		for goType, protoType := range basic {
			result[goType] = protoType
			for _, prefix := range []string{
				"*",
			} {
				result[prefix+goType] = protoType
			}
		}
		return result
	}()
)

func Unmarshal(data any) (f *File, err error) {
	switch val := data.(type) {
	case *spec.ApiSpec:
		f = &File{
			Syntax:  Version3,
			Package: strings.ToLower(val.Service.Name),
			Options: []*Option{
				{
					Name:  "go_package",
					Value: "/protoc-gen-go",
				},
			},
			Service: &Service{Name: val.Service.Name},
		}
		messageMap := make(map[string]*Message, len(val.Types))
		for _, typ := range val.Types {
			defineStruct, _ := typ.(spec.DefineStruct)
			var message Message
			message.Name = defineStruct.Name()
			message.Descs = defineStruct.Documents()
			for _, member := range defineStruct.Members {
				var field MessageField
				if err = field.Unmarshal(&member); err != nil {
					return nil, err
				}
				message.Fields = append(message.Fields, &field)
			}
			f.Messages = append(f.Messages, &message)
			messageMap[message.Name] = &message
		}
		for _, group := range val.Service.JoinPrefix().Groups {
			for _, route := range group.Routes {
				var rpc ServiceRpc
				rpc.Name = route.Handler
				rpc.Descs = []string{strings.Trim(route.JoinedDoc(), `"`)}
				if defineStruct, ok := route.RequestType.(spec.DefineStruct); ok {
					rpc.Request = messageMap[defineStruct.Name()]
				} else {
					if _, exist := messageMap[emptyMessage.Name]; !exist {
						messageMap[emptyMessage.Name] = &emptyMessage
						f.Messages = append([]*Message{&emptyMessage}, f.Messages...)
					}
					rpc.Request = &emptyMessage
				}

				if defineStruct, ok := route.ResponseType.(spec.DefineStruct); ok {
					rpc.Response = messageMap[defineStruct.Name()]
				} else {
					if _, exist := messageMap[emptyMessage.Name]; !exist {
						messageMap[emptyMessage.Name] = &emptyMessage
						f.Messages = append([]*Message{&emptyMessage}, f.Messages...)
					}
					rpc.Response = &emptyMessage
				}
				f.Service.Rpcs = append(f.Service.Rpcs, &rpc)
			}
		}
	default:
		return nil, fmt.Errorf("unsupported type %T, only supported *spec.ApiSpec", data)
	}
	return
}

func (v *MessageField) Unmarshal(data any) error {
	switch val := data.(type) {
	case *spec.Member:
		v.Name = val.Name
		v.Descs = val.Docs
		if comment := strings.TrimSpace(strings.TrimPrefix(val.GetComment(), "//")); comment != "" {
			v.Descs = append(v.Descs, comment)
		}
		if name, exist := fieldTypeNameMap[val.Type.Name()]; exist {
			v.TypeName = name
		} else if strings.Contains(val.Type.Name(), "map[") {
			name, err := parseMapField(val.Type.Name())
			if err != nil {
				return fmt.Errorf("parse map field %s falied, %w", val.Name, err)
			}
			v.TypeName = name
		} else {
			v.TypeName = strings.ReplaceAll(strings.ReplaceAll(val.Type.Name(), "*", ""), "[]", "")
		}
		v.Repeated = strings.Contains(val.Type.Name(), "[]")
		// todo: parse member.Tag
	default:
		return fmt.Errorf("unsupported type %T, only supported *spec.Member", data)
	}
	return nil
}

func parseMapField(typeName string) (string, error) {
	before, after, found := strings.Cut(typeName, "map[")
	if !found {
		return "", fmt.Errorf("field type %s is not map", typeName)
	}
	if before != "" || !strings.Contains(after, "]") {
		return "", fmt.Errorf("unsupported field type %s", typeName)
	}
	mapKV := strings.SplitN(after, "]", 2)
	if mapKeyTypeNameMap[mapKV[0]] == "" || strings.Contains(mapKV[1], "[") {
		return "", fmt.Errorf("unsupported field type %s", typeName)
	}
	if fieldTypeNameMap[mapKV[1]] != "" {
		mapKV[1] = fieldTypeNameMap[mapKV[1]]
	} else {
		mapKV[1] = strings.ReplaceAll(mapKV[1], "*", "")
	}
	return fmt.Sprintf("map<%s,%s>", mapKeyTypeNameMap[mapKV[0]], mapKV[1]), nil
}
