package common

import (
	"google.golang.org/protobuf/reflect/protoreflect"
)

func GetHandleFunctionName(message protoreflect.ProtoMessage) string {
	return string(message.ProtoReflect().Descriptor().FullName())
}
