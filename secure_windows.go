//go:build windows

package main

import (
	"encoding/base64"
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

func protectSecret(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	in := []byte(value)
	blob := windows.DataBlob{Size: uint32(len(in)), Data: &in[0]}
	var out windows.DataBlob
	if err := windows.CryptProtectData(&blob, nil, nil, 0, nil, 0, &out); err != nil {
		return "", err
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(out.Data))))
	data := unsafe.Slice(out.Data, out.Size)
	return "dpapi:" + base64.StdEncoding.EncodeToString(data), nil
}

func unprotectSecret(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if len(value) < 7 || value[:6] != "dpapi:" {
		return "", errors.New("凭据未使用 DPAPI 加密，请重新填写")
	}
	data, err := base64.StdEncoding.DecodeString(value[6:])
	if err != nil || len(data) == 0 {
		return "", errors.New("凭据格式无效")
	}
	blob := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&blob, nil, nil, 0, nil, 0, &out); err != nil {
		return "", err
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(out.Data))))
	return string(unsafe.Slice(out.Data, out.Size)), nil
}
