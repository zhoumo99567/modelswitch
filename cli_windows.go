//go:build windows

package main

import (
	"encoding/base64"
	"encoding/binary"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
)

func launchCLIInTerminal(path, directory string) error {
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	script := "$env:Path=" + quote(filepath.Dir(path)) + "+';'+$env:Path; Set-Location -LiteralPath " + quote(directory) + "; & " + quote(path)
	units := utf16.Encode([]rune(script))
	data := make([]byte, len(units)*2)
	for i, u := range units {
		binary.LittleEndian.PutUint16(data[i*2:], u)
	}
	cmd := exec.Command("powershell.exe", "-NoExit", "-EncodedCommand", base64.StdEncoding.EncodeToString(data))
	cmd.Dir = directory
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000010}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
