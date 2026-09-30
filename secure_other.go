//go:build !windows && !darwin

package main

import "errors"

func protectSecret(value string) (string, error) {
	return "", errors.New("此版本仅支持 Windows 和 macOS 凭据加密")
}
func unprotectSecret(value string) (string, error) {
	return "", errors.New("此版本仅支持 Windows 和 macOS 凭据解密")
}
