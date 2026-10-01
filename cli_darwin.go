//go:build darwin

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func launchCLIInTerminal(path, directory string) error {
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	folder := filepath.Join(cache, "ModelSwitcher", "cli-launchers")
	if err = os.MkdirAll(folder, 0700); err != nil {
		return err
	}
	script, err := os.CreateTemp(folder, "launch-*.command")
	if err != nil {
		return err
	}
	name := script.Name()
	// Opening a .command uses the OS file handler and does not require Apple Events permissions.
	_, err = script.WriteString("#!/bin/sh\n/bin/rm -f -- \"$0\"\n" + cliShellScript(path, directory) + "\n")
	if closeErr := script.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Chmod(name, 0700)
	}
	if err != nil {
		_ = os.Remove(name)
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err = exec.CommandContext(ctx, "open", "-a", "Terminal", name).Run(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}
