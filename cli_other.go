//go:build !darwin && !windows

package main

import "errors"

func launchCLIInTerminal(path, directory string) error {
	return errors.New("CLI 启动目前支持 macOS 和 Windows")
}
