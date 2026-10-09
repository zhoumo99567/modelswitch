package main

import (
	"bufio"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed pi_chat_runtime.mjs
var piChatRuntimeSource []byte

type PiChatRuntimeResource struct {
	Name string `json:"name"`
	Path string `json:"path"`
}
type PiChatRuntimeInfo struct {
	Cwd         string                  `json:"cwd"`
	AgentDir    string                  `json:"agentDir"`
	LogPath     string                  `json:"logPath"`
	Skills      []PiChatRuntimeResource `json:"skills"`
	Extensions  []string                `json:"extensions"`
	Tools       []string                `json:"tools"`
	Diagnostics []string                `json:"diagnostics"`
}
type PiChatRuntimeEvent struct {
	SessionID string          `json:"sessionID"`
	Event     json.RawMessage `json:"event"`
}
type piChatWorker struct {
	id, configID, apiKey string
	cmd                  *exec.Cmd
	stdin                io.WriteCloser
	writeMu              sync.Mutex
	ready                chan piChatWorkerReady
	done                 chan struct{}
	emit                 func(PiChatRuntimeEvent)
	logMu                sync.Mutex
	logFile              *os.File
	logPath              string
}
type piChatWorkerReady struct {
	info PiChatRuntimeInfo
	err  error
}

// Find the SDK belonging to the installed CLI. No downloads or npm commands are
// needed, and the runtime uses the same extension API version as ordinary pi.
func resolvePiChatSDK() (node, sdkPath string, err error) {
	cli, err := resolveCLI("pi")
	if err != nil {
		return "", "", err
	}
	roots := []string{filepath.Dir(cli)}
	if resolved, e := filepath.EvalSymlinks(cli); e == nil {
		roots = append(roots, filepath.Dir(resolved))
	}
	for _, root := range roots {
		for dir, depth := root, 0; depth < 6; dir, depth = filepath.Dir(dir), depth+1 {
			candidates := []string{dir}
			for _, scope := range []string{"@earendil-works", "@mariozechner"} {
				candidates = append(candidates, filepath.Join(dir, "node_modules", scope, "pi-coding-agent"), filepath.Join(dir, "lib", "node_modules", scope, "pi-coding-agent"))
			}
			for _, candidate := range candidates {
				data, e := os.ReadFile(filepath.Join(candidate, "package.json"))
				if e != nil {
					continue
				}
				var pkg struct{ Name string }
				if json.Unmarshal(data, &pkg) != nil || (pkg.Name != "@earendil-works/pi-coding-agent" && pkg.Name != "@mariozechner/pi-coding-agent") {
					continue
				}
				sdkPath = filepath.Join(candidate, "dist", "index.js")
				if info, e := os.Stat(sdkPath); e == nil && !info.IsDir() {
					break
				}
				sdkPath = ""
			}
			if sdkPath != "" {
				break
			}
		}
		if sdkPath != "" {
			break
		}
	}
	if sdkPath == "" {
		return "", "", errors.New("内置对话需要 npm 安装的 pi CLI SDK，请安装或更新 @earendil-works/pi-coding-agent")
	}
	node, err = resolveNode()
	if node == "" {
		return "", "", errors.New("内置对话需要 Node.js，请安装 pi CLI 所需的 Node.js 版本")
	}
	return node, sdkPath, nil
}

func (w *piChatWorker) write(value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	select {
	case <-w.done:
		return errors.New("pi 运行时已关闭，请开启新对话")
	default:
	}
	_, err = w.stdin.Write(append(data, '\n'))
	return err
}

func (w *piChatWorker) close() {
	_ = w.write(map[string]any{"type": "dispose"})
	_ = w.stdin.Close()
	select {
	case <-w.done:
	case <-time.After(3 * time.Second):
		_ = w.cmd.Process.Kill()
		<-w.done
	}
	w.logMu.Lock()
	if w.logFile != nil {
		_ = w.logFile.Close()
		w.logFile = nil
	}
	w.logMu.Unlock()
}

func (w *piChatWorker) writeLog(data []byte) {
	w.logMu.Lock()
	defer w.logMu.Unlock()
	if w.logFile == nil {
		return
	}
	_, _ = w.logFile.Write(append(append([]byte(nil), data...), '\n'))
}

func piChatLogFile(id string) (string, *os.File) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", nil
	}
	root := filepath.Join(cache, "ModelSwitcher", "logs")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", nil
	}
	name := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(id)
	if name == "" {
		name = "session"
	}
	path := filepath.Join(root, fmt.Sprintf("pi-chat-%s-%s.jsonl", time.Now().Format("20060102-150405"), name))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return "", nil
	}
	return path, file
}

func startPiChatWorker(id string, c piChatConnection, emit func(PiChatRuntimeEvent)) (*piChatWorker, error) {
	node, sdkPath, err := resolvePiChatSDK()
	if err != nil {
		return nil, err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(cache, "ModelSwitcher", "pi-runtime", fmt.Sprintf("%x.mjs", sha256.Sum256(piChatRuntimeSource)))
	if err = atomicWrite(path, piChatRuntimeSource); err != nil {
		return nil, err
	}
	cwd, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	cmd := hiddenCommand(node, path)
	cmd.Dir = cwd
	cmd.Env = append(dependencyEnvironment(node), "PI_CODING_AGENT_DIR="+piRoot())
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	w := &piChatWorker{id: id, configID: c.config.ID, apiKey: c.apiKey, cmd: cmd, stdin: stdin, ready: make(chan piChatWorkerReady, 1), done: make(chan struct{}), emit: emit}
	w.logPath, w.logFile = piChatLogFile(id)
	if err = cmd.Start(); err != nil {
		_ = stdin.Close()
		w.logMu.Lock()
		if w.logFile != nil {
			_ = w.logFile.Close()
			w.logFile = nil
		}
		w.logMu.Unlock()
		return nil, err
	}
	// Drain extension logs without putting credentials or arbitrary stdout into
	// the UI. Runtime startup failures are sent explicitly as JSON records.
	go func() { _, _ = io.Copy(io.Discard, stderr) }()
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64<<10), 48<<20)
		for scanner.Scan() {
			data := append([]byte(nil), scanner.Bytes()...)
			if w.apiKey != "" {
				// Escape the key exactly as it occurs inside a JSON string.
				key, _ := json.Marshal(w.apiKey)
				data = []byte(strings.ReplaceAll(string(data), string(key[1:len(key)-1]), "[redacted]"))
			}
			var record struct {
				Type    string            `json:"type"`
				Info    PiChatRuntimeInfo `json:"info"`
				Message string            `json:"message"`
			}
			if json.Unmarshal(data, &record) != nil {
				continue // An extension wrote a non-protocol line to stdout.
			}
			if record.Type == "diagnostic" || record.Type == "recovery" {
				w.writeLog(data)
				emit(PiChatRuntimeEvent{SessionID: id, Event: data})
			} else if record.Type == "ready" || record.Type == "fatal" {
				record.Info.LogPath = w.logPath
				result := piChatWorkerReady{info: record.Info}
				if record.Type == "fatal" {
					result.err = errors.New(record.Message)
				}
				select {
				case w.ready <- result:
				default:
				}
			} else {
				emit(PiChatRuntimeEvent{SessionID: id, Event: data})
			}
		}
		if scanner.Err() != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
		close(w.done)
		data, _ := json.Marshal(map[string]any{"type": "closed", "message": "pi 运行时已退出，请开启新对话；确认 pi CLI 和 Node.js 可以正常运行。"})
		emit(PiChatRuntimeEvent{SessionID: id, Event: data})
	}()
	if err = w.write(map[string]any{"type": "init", "sdkPath": sdkPath, "agentDir": piRoot(), "cwd": cwd, "logPath": w.logPath, "model": c.config.Model, "apiKey": c.apiKey, "headers": c.headers}); err != nil {
		w.close()
		return nil, err
	}
	return w, nil
}

func (a *App) emitPiChatRuntime(event PiChatRuntimeEvent) {
	if a.piRuntimeEmit != nil {
		a.piRuntimeEmit(event)
	} else if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "pi-chat-runtime", event)
	}
}

func (a *App) OpenPiChatRuntime(id, configID string) (PiChatRuntimeInfo, error) {
	a.piRuntimeOpenMu.Lock()
	defer a.piRuntimeOpenMu.Unlock()
	if id == "" || len(id) > 128 {
		return PiChatRuntimeInfo{}, errors.New("对话会话标识无效")
	}
	a.mu.Lock()
	c, err := loadPiChatConnection()
	if err == nil {
		err = ensurePiMemoryBridge()
	}
	a.mu.Unlock()
	if err != nil {
		return PiChatRuntimeInfo{}, err
	}
	if c.config.ID != configID {
		return PiChatRuntimeInfo{}, errors.New("当前配置已变化，请刷新对话")
	}
	a.piRuntimeMu.Lock()
	if a.piRuntime != nil {
		a.piRuntime.close()
		a.piRuntime = nil
	}
	w, err := startPiChatWorker(id, c, a.emitPiChatRuntime)
	if err != nil {
		a.piRuntimeMu.Unlock()
		return PiChatRuntimeInfo{}, err
	}
	a.piRuntime = w
	a.piRuntimeMu.Unlock()
	select {
	case result := <-w.ready:
		if result.err == nil {
			return result.info, nil
		}
		err = result.err
	case <-w.done:
		err = errors.New("pi 运行时启动失败，请确认 pi CLI 和 Node.js 版本兼容")
	case <-time.After(45 * time.Second):
		err = errors.New("加载 pi 全局资源超时，请检查 extensions 和 pi CLI")
	}
	w.close()
	a.piRuntimeMu.Lock()
	if a.piRuntime == w {
		a.piRuntime = nil
	}
	a.piRuntimeMu.Unlock()
	return PiChatRuntimeInfo{}, err
}

func (a *App) PiChatRuntimeCommand(id, command string) error {
	if len(command) > 32<<20 {
		return errors.New("对话及图片总大小超过 32 MB，请减少附件或开启新对话")
	}
	var input map[string]json.RawMessage
	if err := json.Unmarshal([]byte(command), &input); err != nil {
		return errors.New("pi 对话命令无效")
	}
	kind := rawString(input, "type")
	if kind != "run" && kind != "abort" && kind != "ui_response" && kind != "editor_state" {
		return errors.New("不支持的 pi 对话命令")
	}
	if kind == "run" {
		a.mu.Lock()
		c, err := loadPiChatConnection()
		a.mu.Unlock()
		if err != nil {
			return err
		}
		a.piRuntimeMu.Lock()
		defer a.piRuntimeMu.Unlock()
		if a.piRuntime == nil || a.piRuntime.id != id || a.piRuntime.configID != c.config.ID {
			return errors.New("当前配置已变化，请开启新对话")
		}
		return a.piRuntime.write(input)
	}
	a.piRuntimeMu.Lock()
	defer a.piRuntimeMu.Unlock()
	if a.piRuntime == nil || a.piRuntime.id != id {
		return nil
	}
	return a.piRuntime.write(input)
}

func (a *App) ClosePiChatRuntime(id string) {
	a.piRuntimeMu.Lock()
	defer a.piRuntimeMu.Unlock()
	if a.piRuntime != nil && (id == "" || a.piRuntime.id == id) {
		a.piRuntime.close()
		a.piRuntime = nil
	}
}
