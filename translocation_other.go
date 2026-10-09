//go:build !darwin || !cgo

package main

func originalExecutablePath(string) (string, bool) { return "", false }
