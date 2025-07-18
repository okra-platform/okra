// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        (unknown)
// source: internal/runtime/pb/runtime.proto

package pb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	anypb "google.golang.org/protobuf/types/known/anypb"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// ServiceRequest represents a request to execute a service method
type ServiceRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Unique identifier for this request
	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	// Name of the service method to invoke
	Method string `protobuf:"bytes,2,opt,name=method,proto3" json:"method,omitempty"`
	// JSON-encoded input for the method
	Input []byte `protobuf:"bytes,3,opt,name=input,proto3" json:"input,omitempty"`
	// Optional request metadata
	Metadata map[string]string `protobuf:"bytes,4,rep,name=metadata,proto3" json:"metadata,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	// Maximum time for execution
	Timeout       *durationpb.Duration `protobuf:"bytes,5,opt,name=timeout,proto3" json:"timeout,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ServiceRequest) Reset() {
	*x = ServiceRequest{}
	mi := &file_internal_runtime_pb_runtime_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ServiceRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ServiceRequest) ProtoMessage() {}

func (x *ServiceRequest) ProtoReflect() protoreflect.Message {
	mi := &file_internal_runtime_pb_runtime_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ServiceRequest.ProtoReflect.Descriptor instead.
func (*ServiceRequest) Descriptor() ([]byte, []int) {
	return file_internal_runtime_pb_runtime_proto_rawDescGZIP(), []int{0}
}

func (x *ServiceRequest) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *ServiceRequest) GetMethod() string {
	if x != nil {
		return x.Method
	}
	return ""
}

func (x *ServiceRequest) GetInput() []byte {
	if x != nil {
		return x.Input
	}
	return nil
}

func (x *ServiceRequest) GetMetadata() map[string]string {
	if x != nil {
		return x.Metadata
	}
	return nil
}

func (x *ServiceRequest) GetTimeout() *durationpb.Duration {
	if x != nil {
		return x.Timeout
	}
	return nil
}

// ServiceResponse represents the response from a service method execution
type ServiceResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Matches the request ID
	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	// Indicates if the request was successful
	Success bool `protobuf:"varint,2,opt,name=success,proto3" json:"success,omitempty"`
	// JSON-encoded output from the method (if successful)
	Output []byte `protobuf:"bytes,3,opt,name=output,proto3" json:"output,omitempty"`
	// Error information (if not successful)
	Error *ServiceError `protobuf:"bytes,4,opt,name=error,proto3" json:"error,omitempty"`
	// Optional response metadata
	Metadata map[string]string `protobuf:"bytes,5,rep,name=metadata,proto3" json:"metadata,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	// How long the execution took
	Duration      *durationpb.Duration `protobuf:"bytes,6,opt,name=duration,proto3" json:"duration,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ServiceResponse) Reset() {
	*x = ServiceResponse{}
	mi := &file_internal_runtime_pb_runtime_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ServiceResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ServiceResponse) ProtoMessage() {}

func (x *ServiceResponse) ProtoReflect() protoreflect.Message {
	mi := &file_internal_runtime_pb_runtime_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ServiceResponse.ProtoReflect.Descriptor instead.
func (*ServiceResponse) Descriptor() ([]byte, []int) {
	return file_internal_runtime_pb_runtime_proto_rawDescGZIP(), []int{1}
}

func (x *ServiceResponse) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *ServiceResponse) GetSuccess() bool {
	if x != nil {
		return x.Success
	}
	return false
}

func (x *ServiceResponse) GetOutput() []byte {
	if x != nil {
		return x.Output
	}
	return nil
}

func (x *ServiceResponse) GetError() *ServiceError {
	if x != nil {
		return x.Error
	}
	return nil
}

func (x *ServiceResponse) GetMetadata() map[string]string {
	if x != nil {
		return x.Metadata
	}
	return nil
}

func (x *ServiceResponse) GetDuration() *durationpb.Duration {
	if x != nil {
		return x.Duration
	}
	return nil
}

// ServiceError represents an error from service execution
type ServiceError struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Error code (e.g., "VALIDATION_ERROR", "EXECUTION_ERROR")
	Code string `protobuf:"bytes,1,opt,name=code,proto3" json:"code,omitempty"`
	// Human-readable error message
	Message string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
	// Additional error context
	Details       map[string]*anypb.Any `protobuf:"bytes,3,rep,name=details,proto3" json:"details,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ServiceError) Reset() {
	*x = ServiceError{}
	mi := &file_internal_runtime_pb_runtime_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ServiceError) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ServiceError) ProtoMessage() {}

func (x *ServiceError) ProtoReflect() protoreflect.Message {
	mi := &file_internal_runtime_pb_runtime_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ServiceError.ProtoReflect.Descriptor instead.
func (*ServiceError) Descriptor() ([]byte, []int) {
	return file_internal_runtime_pb_runtime_proto_rawDescGZIP(), []int{2}
}

func (x *ServiceError) GetCode() string {
	if x != nil {
		return x.Code
	}
	return ""
}

func (x *ServiceError) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

func (x *ServiceError) GetDetails() map[string]*anypb.Any {
	if x != nil {
		return x.Details
	}
	return nil
}

// HealthCheck is used to verify actor is alive
type HealthCheck struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Ping          string                 `protobuf:"bytes,1,opt,name=ping,proto3" json:"ping,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *HealthCheck) Reset() {
	*x = HealthCheck{}
	mi := &file_internal_runtime_pb_runtime_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *HealthCheck) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HealthCheck) ProtoMessage() {}

func (x *HealthCheck) ProtoReflect() protoreflect.Message {
	mi := &file_internal_runtime_pb_runtime_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HealthCheck.ProtoReflect.Descriptor instead.
func (*HealthCheck) Descriptor() ([]byte, []int) {
	return file_internal_runtime_pb_runtime_proto_rawDescGZIP(), []int{3}
}

func (x *HealthCheck) GetPing() string {
	if x != nil {
		return x.Ping
	}
	return ""
}

// HealthCheckResponse confirms actor is alive
type HealthCheckResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Pong          string                 `protobuf:"bytes,1,opt,name=pong,proto3" json:"pong,omitempty"`
	Ready         bool                   `protobuf:"varint,2,opt,name=ready,proto3" json:"ready,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *HealthCheckResponse) Reset() {
	*x = HealthCheckResponse{}
	mi := &file_internal_runtime_pb_runtime_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *HealthCheckResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HealthCheckResponse) ProtoMessage() {}

func (x *HealthCheckResponse) ProtoReflect() protoreflect.Message {
	mi := &file_internal_runtime_pb_runtime_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HealthCheckResponse.ProtoReflect.Descriptor instead.
func (*HealthCheckResponse) Descriptor() ([]byte, []int) {
	return file_internal_runtime_pb_runtime_proto_rawDescGZIP(), []int{4}
}

func (x *HealthCheckResponse) GetPong() string {
	if x != nil {
		return x.Pong
	}
	return ""
}

func (x *HealthCheckResponse) GetReady() bool {
	if x != nil {
		return x.Ready
	}
	return false
}

var File_internal_runtime_pb_runtime_proto protoreflect.FileDescriptor

const file_internal_runtime_pb_runtime_proto_rawDesc = "" +
	"\n" +
	"!internal/runtime/pb/runtime.proto\x12\aruntime\x1a\x19google/protobuf/any.proto\x1a\x1egoogle/protobuf/duration.proto\"\x83\x02\n" +
	"\x0eServiceRequest\x12\x0e\n" +
	"\x02id\x18\x01 \x01(\tR\x02id\x12\x16\n" +
	"\x06method\x18\x02 \x01(\tR\x06method\x12\x14\n" +
	"\x05input\x18\x03 \x01(\fR\x05input\x12A\n" +
	"\bmetadata\x18\x04 \x03(\v2%.runtime.ServiceRequest.MetadataEntryR\bmetadata\x123\n" +
	"\atimeout\x18\x05 \x01(\v2\x19.google.protobuf.DurationR\atimeout\x1a;\n" +
	"\rMetadataEntry\x12\x10\n" +
	"\x03key\x18\x01 \x01(\tR\x03key\x12\x14\n" +
	"\x05value\x18\x02 \x01(\tR\x05value:\x028\x01\"\xb8\x02\n" +
	"\x0fServiceResponse\x12\x0e\n" +
	"\x02id\x18\x01 \x01(\tR\x02id\x12\x18\n" +
	"\asuccess\x18\x02 \x01(\bR\asuccess\x12\x16\n" +
	"\x06output\x18\x03 \x01(\fR\x06output\x12+\n" +
	"\x05error\x18\x04 \x01(\v2\x15.runtime.ServiceErrorR\x05error\x12B\n" +
	"\bmetadata\x18\x05 \x03(\v2&.runtime.ServiceResponse.MetadataEntryR\bmetadata\x125\n" +
	"\bduration\x18\x06 \x01(\v2\x19.google.protobuf.DurationR\bduration\x1a;\n" +
	"\rMetadataEntry\x12\x10\n" +
	"\x03key\x18\x01 \x01(\tR\x03key\x12\x14\n" +
	"\x05value\x18\x02 \x01(\tR\x05value:\x028\x01\"\xcc\x01\n" +
	"\fServiceError\x12\x12\n" +
	"\x04code\x18\x01 \x01(\tR\x04code\x12\x18\n" +
	"\amessage\x18\x02 \x01(\tR\amessage\x12<\n" +
	"\adetails\x18\x03 \x03(\v2\".runtime.ServiceError.DetailsEntryR\adetails\x1aP\n" +
	"\fDetailsEntry\x12\x10\n" +
	"\x03key\x18\x01 \x01(\tR\x03key\x12*\n" +
	"\x05value\x18\x02 \x01(\v2\x14.google.protobuf.AnyR\x05value:\x028\x01\"!\n" +
	"\vHealthCheck\x12\x12\n" +
	"\x04ping\x18\x01 \x01(\tR\x04ping\"?\n" +
	"\x13HealthCheckResponse\x12\x12\n" +
	"\x04pong\x18\x01 \x01(\tR\x04pong\x12\x14\n" +
	"\x05ready\x18\x02 \x01(\bR\x05readyB3Z1github.com/okra-platform/okra/internal/runtime/pbb\x06proto3"

var (
	file_internal_runtime_pb_runtime_proto_rawDescOnce sync.Once
	file_internal_runtime_pb_runtime_proto_rawDescData []byte
)

func file_internal_runtime_pb_runtime_proto_rawDescGZIP() []byte {
	file_internal_runtime_pb_runtime_proto_rawDescOnce.Do(func() {
		file_internal_runtime_pb_runtime_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_internal_runtime_pb_runtime_proto_rawDesc), len(file_internal_runtime_pb_runtime_proto_rawDesc)))
	})
	return file_internal_runtime_pb_runtime_proto_rawDescData
}

var file_internal_runtime_pb_runtime_proto_msgTypes = make([]protoimpl.MessageInfo, 8)
var file_internal_runtime_pb_runtime_proto_goTypes = []any{
	(*ServiceRequest)(nil),      // 0: runtime.ServiceRequest
	(*ServiceResponse)(nil),     // 1: runtime.ServiceResponse
	(*ServiceError)(nil),        // 2: runtime.ServiceError
	(*HealthCheck)(nil),         // 3: runtime.HealthCheck
	(*HealthCheckResponse)(nil), // 4: runtime.HealthCheckResponse
	nil,                         // 5: runtime.ServiceRequest.MetadataEntry
	nil,                         // 6: runtime.ServiceResponse.MetadataEntry
	nil,                         // 7: runtime.ServiceError.DetailsEntry
	(*durationpb.Duration)(nil), // 8: google.protobuf.Duration
	(*anypb.Any)(nil),           // 9: google.protobuf.Any
}
var file_internal_runtime_pb_runtime_proto_depIdxs = []int32{
	5, // 0: runtime.ServiceRequest.metadata:type_name -> runtime.ServiceRequest.MetadataEntry
	8, // 1: runtime.ServiceRequest.timeout:type_name -> google.protobuf.Duration
	2, // 2: runtime.ServiceResponse.error:type_name -> runtime.ServiceError
	6, // 3: runtime.ServiceResponse.metadata:type_name -> runtime.ServiceResponse.MetadataEntry
	8, // 4: runtime.ServiceResponse.duration:type_name -> google.protobuf.Duration
	7, // 5: runtime.ServiceError.details:type_name -> runtime.ServiceError.DetailsEntry
	9, // 6: runtime.ServiceError.DetailsEntry.value:type_name -> google.protobuf.Any
	7, // [7:7] is the sub-list for method output_type
	7, // [7:7] is the sub-list for method input_type
	7, // [7:7] is the sub-list for extension type_name
	7, // [7:7] is the sub-list for extension extendee
	0, // [0:7] is the sub-list for field type_name
}

func init() { file_internal_runtime_pb_runtime_proto_init() }
func file_internal_runtime_pb_runtime_proto_init() {
	if File_internal_runtime_pb_runtime_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_internal_runtime_pb_runtime_proto_rawDesc), len(file_internal_runtime_pb_runtime_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   8,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_internal_runtime_pb_runtime_proto_goTypes,
		DependencyIndexes: file_internal_runtime_pb_runtime_proto_depIdxs,
		MessageInfos:      file_internal_runtime_pb_runtime_proto_msgTypes,
	}.Build()
	File_internal_runtime_pb_runtime_proto = out.File
	file_internal_runtime_pb_runtime_proto_goTypes = nil
	file_internal_runtime_pb_runtime_proto_depIdxs = nil
}
