//go:build windows

package main

import (
	"encoding/base64"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
)

func launchCLIInTerminal(path, directory string) error {
	process, err := startCLIHost(path, directory)
	if err != nil {
		return err
	}
	return process.Release()
}

func startCLIHost(path, directory string) (*os.Process, error) {
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	// Keep the host open after a failed CLI start so users can see the actual
	// error instead of getting a terminal window that flashes and disappears.
	script := "$ErrorActionPreference='Stop'; $env:Path=" + quote(filepath.Dir(path)) + "+';'+$env:Path; Set-Location -LiteralPath " + quote(directory) + "; try { & " + quote(path) + "; if ($LASTEXITCODE -and $LASTEXITCODE -ne 0) { Write-Host ('CLI exited with code ' + $LASTEXITCODE) -ForegroundColor Red; Read-Host 'Press Enter to close this window' } } catch { Write-Host $_ -ForegroundColor Red; Read-Host 'Press Enter to close this window' }"
	units := utf16.Encode([]rune(script))
	data := make([]byte, len(units)*2)
	for i, u := range units {
		binary.LittleEndian.PutUint16(data[i*2:], u)
	}
	cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NoExit", "-EncodedCommand", base64.StdEncoding.EncodeToString(data))
	cmd.Dir = directory
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000010, NoInheritHandles: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd.Process, nil
}
