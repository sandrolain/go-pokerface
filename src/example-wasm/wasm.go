package main

import (
	"strings"
	"unsafe"

	"github.com/francoispqt/gojay"
	"github.com/sandrolain/go-pokerface/src/pokerface/shared"
)

func main() {

}

//export alloc
func alloc(size uint32) *byte {
	buf := make([]byte, size)
	return &buf[0]
}

func bytesToPtr(buf []byte) uint64 {
	bufPtr := &buf[0]
	unsafePtr := uintptr(unsafe.Pointer(bufPtr))
	ptr := uint32(unsafePtr)
	size := uint32(len(buf))
	return (uint64(ptr) << uint64(32)) | uint64(size)
}

func strToPtr(s string) uint64 {
	return bytesToPtr([]byte(s))
}

func ptrToStr(subject *uint32, length int) string {
	var subjectStr strings.Builder
	pointer := uintptr(unsafe.Pointer(subject))
	for i := 0; i < length; i++ {
		s := *(*int32)(unsafe.Pointer(pointer + uintptr(i)))
		subjectStr.WriteByte(byte(s))
	}
	return subjectStr.String()
}

func ptrToBytes(subject *uint32, length uint32) []byte {
	res := make([]byte, length)
	pointer := uintptr(unsafe.Pointer(subject))
	for i := uint32(0); i < length; i++ {
		s := *(*int32)(unsafe.Pointer(pointer + uintptr(i)))
		res[i] = byte(s)
	}
	return res
}

//export filter
func filter(ptr *uint32, length uint32) (ptrAndSize uint64) {
	reqJson := ptrToBytes(ptr, length)

	req := shared.RequestInfo{}
	err := gojay.Unmarshal(reqJson, &req)
	if err != nil {
		panic("cannot unmarshal")
	}

	req.Method = "POST"
	req.Path = "/foo/bar"

	req.Headers["From-Filter"] = []string{"Hello World"}
	delete(req.Headers, "User-Agent")

	req.Cookies["foo"] = "bar"
	delete(req.Cookies, "auth")

	resJson, err := gojay.MarshalJSONObject(&req)
	if err != nil {
		return
	}
	return bytesToPtr(resJson)
}
