//go:build !darwin && !windows

package main

import "errors"

func nativeWindowState() (windowBounds, []windowDisplay, bool, error) {
	return windowBounds{}, nil, false, errors.New("当前平台未提供窗口坐标")
}
func nativeApplyWindowBounds(windowBounds) error {
	return errors.New("当前平台未提供窗口坐标")
}
