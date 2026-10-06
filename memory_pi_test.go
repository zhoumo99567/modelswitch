package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the installed extension's actual before_agent_start callback without
// contacting a model service or depending on the globally installed Pi version.
func piMemoryPrompt(t *testing.T, cwd string) string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required to exercise the Pi extension")
	}
	const script = `
import { pathToFileURL } from "node:url";
const { default: register } = await import(pathToFileURL(process.argv[1]).href);
let handler;
register({ on(event, callback) { if (event === "before_agent_start") handler = callback; } });
if (!handler) throw new Error("Shared memory extension did not register a prompt handler");
const result = await handler({ systemPrompt: "Base Pi instructions", prompt: "你叫什么名字？" }, { cwd: process.argv[2], hasUI: false });
console.log(JSON.stringify(result?.systemPrompt ?? "Base Pi instructions"));
`
	output, err := exec.Command(node, "--input-type=module", "--eval", script, filepath.Join(piRoot(), "extensions", "model-switcher-memory.js"), cwd).Output()
	if err != nil {
		if failure, ok := err.(*exec.ExitError); ok {
			t.Fatalf("Pi shared memory prompt failed: %v\n%s", err, failure.Stderr)
		}
		t.Fatalf("Pi shared memory prompt failed: %v\n%s", err, output)
	}
	var prompt string
	if err := json.Unmarshal(output, &prompt); err != nil {
		t.Fatalf("decode Pi prompt: %v\n%s", err, output)
	}
	return prompt
}

func TestPiMemoryBridgeTracksMemoryEditsAndDeletion(t *testing.T) {
	isolateAgentHomes(t)
	app := NewApp()
	entry, err := app.CreateMemory("soul.md", "你的名字: 初始名字\n", "global")
	if err != nil {
		t.Fatal(err)
	}
	if err := app.WriteMemory(entry.Path, "你的名字: 更新名字\n"); err != nil {
		t.Fatal(err)
	}
	cwd := t.TempDir()
	prompt := piMemoryPrompt(t, cwd)
	if !strings.Contains(prompt, "更新名字") || strings.Contains(prompt, "初始名字") {
		t.Fatalf("Pi must read the updated memory: %s", prompt)
	}
	if err := app.DeleteMemory(entry.Path); err != nil {
		t.Fatal(err)
	}
	if prompt := piMemoryPrompt(t, cwd); prompt != "Base Pi instructions" {
		t.Fatalf("deleted memories must leave the base instructions intact: %s", prompt)
	}
}

func TestPiMemoryBridgeReloadsWithinSameSession(t *testing.T) {
	isolateAgentHomes(t)
	entry, err := NewApp().CreateMemory("soul.md", "旧名字\n", "global")
	if err != nil {
		t.Fatal(err)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required to exercise the Pi extension")
	}
	const script = `
import { pathToFileURL } from "node:url";
import { writeFileSync, unlinkSync } from "node:fs";
const { default: register } = await import(pathToFileURL(process.argv[1]).href);
let handler;
register({ on(event, callback) { if (event === "before_agent_start") handler = callback; } });
const ctx = { cwd: process.argv[2], hasUI: false };
let systemPrompt = "Base Pi instructions";
const prompts = [];
for (let turn = 0; turn < 3; turn++) {
  if (turn === 1) writeFileSync(process.argv[3], "新名字\n");
  if (turn === 2) unlinkSync(process.argv[3]);
  const result = await handler({ systemPrompt, prompt: "你叫什么名字？" }, ctx);
  systemPrompt = result.systemPrompt;
  prompts.push(systemPrompt);
}
console.log(JSON.stringify(prompts));
`
	output, err := exec.Command(node, "--input-type=module", "--eval", script, filepath.Join(piRoot(), "extensions", piMemoryBridgeName), t.TempDir(), entry.Path).Output()
	if err != nil {
		t.Fatal(err)
	}
	var prompts []string
	if err := json.Unmarshal(output, &prompts); err != nil {
		t.Fatal(err)
	}
	if len(prompts) != 3 || !strings.Contains(prompts[0], "旧名字") || !strings.Contains(prompts[1], "新名字") || strings.Contains(prompts[1], "旧名字") || prompts[2] != "Base Pi instructions" {
		t.Fatalf("memories were stale across turns: %#v", prompts)
	}
	if strings.Count(prompts[1], "<!-- model-switcher:shared-memory:start -->") != 1 {
		t.Fatal("memory context was duplicated across turns")
	}
}

func TestPiMemoryBridgeSelectsCurrentProjectAndAncestors(t *testing.T) {
	isolateAgentHomes(t)
	project := t.TempDir()
	nested := filepath.Join(project, "child")
	if err := os.MkdirAll(nested, 0700); err != nil {
		t.Fatal(err)
	}
	writeMemoryFile(t, filepath.Join(memoryVaultGlobalDir(), "soul.md"), "全局名字\n")
	writeMemoryFile(t, filepath.Join(memoryProjectDir(project), "context.md"), "当前父项目\n")
	writeMemoryFile(t, filepath.Join(memoryProjectDir(nested), "context.md"), "当前子项目\n")
	writeMemoryFile(t, filepath.Join(memoryProjectDir(t.TempDir()), "context.md"), "其他项目不能加载\n")
	if err := refreshMemoryVaultIndex(); err != nil {
		t.Fatal(err)
	}
	prompt := piMemoryPrompt(t, nested)
	for _, want := range []string{"全局名字", "当前父项目", "当前子项目"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("Pi prompt missing %q: %s", want, prompt)
		}
	}
	if strings.Contains(prompt, "其他项目不能加载") || strings.Index(prompt, "当前子项目") < strings.Index(prompt, "当前父项目") {
		t.Fatalf("project selection or precedence is wrong: %s", prompt)
	}
}

func TestPiMemoryBridgeBoundsContextAndPrioritizesSoul(t *testing.T) {
	isolateAgentHomes(t)
	writeMemoryFile(t, filepath.Join(memoryVaultGlobalDir(), "a-large.md"), strings.Repeat("a", 256<<10))
	writeMemoryFile(t, filepath.Join(memoryVaultGlobalDir(), "SOUL.MD"), "名字必须保留\n")
	if err := refreshMemoryVaultIndex(); err != nil {
		t.Fatal(err)
	}
	prompt := piMemoryPrompt(t, t.TempDir())
	if !strings.Contains(prompt, "名字必须保留") || !strings.Contains(prompt, "Excerpt") || len(prompt) > 70<<10 {
		t.Fatalf("memory context was not prioritized and bounded: %d bytes", len(prompt))
	}
}

func TestEnsurePiMemoryBridgeMigratesExistingVaultWithoutChangingPrompts(t *testing.T) {
	isolateAgentHomes(t)
	if err := ensurePiMemoryBridge(); err != nil {
		t.Fatal(err)
	}
	bridgePath := filepath.Join(piRoot(), "extensions", piMemoryBridgeName)
	if _, err := os.Stat(bridgePath); !os.IsNotExist(err) {
		t.Fatalf("an absent vault must not install an extension: %v", err)
	}
	writeMemoryFile(t, filepath.Join(memoryVaultGlobalDir(), "soul.md"), "已有名字\n")
	promptPath := filepath.Join(piRoot(), "APPEND_SYSTEM.md")
	writeMemoryFile(t, promptPath, "用户自己的指令\n")
	if _, err := NewApp().LoadAdvancedState(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(piMemoryPrompt(t, t.TempDir()), "已有名字") {
		t.Fatal("loading advanced settings must connect a pre-existing vault")
	}
	data, err := os.ReadFile(promptPath)
	if err != nil || string(data) != "用户自己的指令\n" {
		t.Fatalf("user instructions changed: %q, %v", data, err)
	}
	before, err := os.Stat(bridgePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensurePiMemoryBridge(); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(bridgePath)
	if err != nil || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("unchanged integration must not be rewritten")
	}
	writeMemoryFile(t, bridgePath, "// 用户自己的扩展\n")
	if err := ensurePiMemoryBridge(); err == nil {
		t.Fatal("must not overwrite an extension owned by the user")
	}
	data, _ = os.ReadFile(bridgePath)
	if string(data) != "// 用户自己的扩展\n" {
		t.Fatal("overwrote the user extension")
	}
}

func TestPiMemoryBridgeLoadsSoul(t *testing.T) {
	isolateAgentHomes(t)
	app := NewApp()
	const soul = "你的名字: 蛋炒饭\n你的性别: 无特殊性别\n"
	if _, err := app.CreateMemory("soul.md", soul, "global"); err != nil {
		t.Fatal(err)
	}
	prompt := piMemoryPrompt(t, t.TempDir())
	for _, want := range []string{"Base Pi instructions", "soul.md", strings.TrimSpace(soul)} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("Pi prompt does not contain %q:\n%s", want, prompt)
		}
	}
}
