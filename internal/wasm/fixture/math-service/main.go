//go:build wasi || wasip1
// +build wasi wasip1

package main

// #include <stdlib.h>
import "C"
import (
	"encoding/json"
	"unsafe"
)

// mathServiceImpl implements the MathService interface
type mathServiceImpl struct{}

// Add implements the add method
func (s *mathServiceImpl) Add(input *AddInput) (*AddResponse, error) {
	return &AddResponse{
		Sum: input.A + input.B,
	}, nil
}

var service MathService

func main() {
	// Reactor module - main should not be called
}

//export _initialize
func _initialize() {
	service = &mathServiceImpl{}
}

//export handle_request
func handle_request(methodPtr, methodLen, inputPtr, inputLen uint32) uint64 {
	method := ptrToString(methodPtr, methodLen)
	input := ptrToBytes(inputPtr, inputLen)

	var output []byte

	switch method {
	case "add":
		var req AddInput
		if err := json.Unmarshal(input, &req); err != nil {
			return 0
		}
		res, err := service.Add(&req)
		if err != nil {
			return 0
		}
		output, err = json.Marshal(res)
		if err != nil {
			return 0
		}
	default:
		return 0
	}

	// Allocate memory for output
	ptr := allocate(uint32(len(output)))
	copy(ptrToBytes(ptr, uint32(len(output))), output)

	// Return pointer and length as uint64 (ptr << 32 | len)
	return uint64(ptr)<<32 | uint64(len(output))
}

//export allocate
func allocate(size uint32) uint32 {
	ptr := C.malloc(C.size_t(size))
	return uint32(uintptr(ptr))
}

//export deallocate
func deallocate(ptr uint32) {
	C.free(unsafe.Pointer(uintptr(ptr)))
}

// Helper functions
func ptrToString(ptr, len uint32) string {
	return string(ptrToBytes(ptr, len))
}

func ptrToBytes(ptr, len uint32) []byte {
	if len == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), len)
}