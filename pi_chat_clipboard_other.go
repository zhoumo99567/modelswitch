//go:build !windows && !darwin

package main

func piChatClipboardPaths() ([]string, error) { return nil, nil }
