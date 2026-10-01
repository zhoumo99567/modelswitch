package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const providerID = "model_switcher_local"
const managedStart = "# >>> MODELSWITCHER MANAGED >>>"
const managedEnd = "# <<< MODELSWITCHER MANAGED <<<"

func parseConfig(data []byte) (map[string]any, error) {
	var result map[string]any
	if err := toml.Unmarshal(data, &result); err != nil {
		return nil, errors.New("Codex 配置不是有效 TOML，未进行任何修改")
	}
	if result == nil {
		result = map[string]any{}
	}
	for _, key := range []string{"model", "model_provider"} {
		if v, ok := result[key]; ok {
			if _, valid := v.(string); !valid {
				return nil, fmt.Errorf("%s 必须是字符串", key)
			}
		}
	}
	return result, nil
}
func stringValue(data map[string]any, key string) string {
	value, _ := data[key].(string)
	return value
}
func optionalString(data map[string]any, key string) *string {
	value, ok := data[key].(string)
	if !ok {
		return nil
	}
	return &value
}
func readConfig() ([]byte, error) {
	data, err := os.ReadFile(configPath())
	if errors.Is(err, os.ErrNotExist) {
		return []byte{}, nil
	}
	return data, err
}
func configuredProfileID(config map[string]any) string {
	providers, _ := config["model_providers"].(map[string]any)
	provider, _ := providers[providerID].(map[string]any)
	name := stringValue(provider, "name")
	parts := strings.SplitN(name, " / ", 3)
	if len(parts) == 3 && parts[0] == "ModelSwitcher" {
		return parts[1]
	}
	return ""
}
func removeManaged(content string) (string, error) {
	start := strings.Index(content, managedStart)
	end := strings.Index(content, managedEnd)
	if start < 0 && end < 0 {
		return content, nil
	}
	if start < 0 || end < start || strings.Count(content, managedStart) != 1 || strings.Count(content, managedEnd) != 1 {
		return "", errors.New("工具管理段标记不完整，请从备份恢复后重试")
	}
	return content[:start] + content[end+len(managedEnd):], nil
}

// Root-only editing keeps nested profile settings, comments and unrelated tables intact.
// Parse and compare semantics afterwards to reject unusual TOML we cannot safely edit.
func setRoot(content, key string, value *string) (string, error) {
	before, err := parseConfig([]byte(content))
	if err != nil {
		return "", err
	}
	boundary := regexp.MustCompile("(?m)^[ \t]*\\[").FindStringIndex(content)
	cut := len(content)
	if boundary != nil {
		cut = boundary[0]
	}
	root, tail := content[:cut], content[cut:]
	re := regexp.MustCompile("(?m)^[ \t]*(?:" + key + "|\"" + key + "\"|'" + key + "')[ \t]*=[^\r\n]*(?:\r?\n|$)")
	matches := re.FindAllStringIndex(root, -1)
	if _, exists := before[key]; exists && len(matches) != 1 {
		return "", fmt.Errorf("无法安全编辑 %s；未修改原配置", key)
	}
	root = re.ReplaceAllString(root, "")
	if value != nil {
		root = key + " = " + strconv.Quote(*value) + "\n" + root
		before[key] = *value
	} else {
		delete(before, key)
	}
	result := root + tail
	after, err := parseConfig([]byte(result))
	if err != nil {
		return "", err
	}
	if !reflect.DeepEqual(before, after) {
		return "", errors.New("配置校验发现无关字段变化，已取消写入")
	}
	return result, nil
}
func localConfig(original []byte, profile storedProfile, helper string) ([]byte, error) {
	content, err := removeManaged(string(original))
	if err != nil {
		return nil, err
	}
	parsed, err := parseConfig([]byte(content))
	if err != nil {
		return nil, err
	}
	providers, _ := parsed["model_providers"].(map[string]any)
	if _, exists := providers[providerID]; exists {
		return nil, errors.New("已有同名模型提供方，无法安全覆盖")
	}
	content, err = setRoot(content, "model", &profile.SelectedModel)
	if err != nil {
		return nil, err
	}
	provider := providerID
	content, err = setRoot(content, "model_provider", &provider)
	if err != nil {
		return nil, err
	}
	name := "ModelSwitcher / " + profile.ID + " / " + profile.Name
	section := fmt.Sprintf("\n%s\n[model_providers.%s]\nname = %s\nbase_url = %s\nwire_api = \"responses\"\n", managedStart, providerID, strconv.Quote(name), strconv.Quote(profile.BaseURL))
	if profile.Secret != "" {
		if helper == "" {
			return nil, errors.New("缺少凭据助手")
		}
		section += fmt.Sprintf("\n[model_providers.%s.auth]\ncommand = %s\nargs = [\"--model-switcher-token\", %s]\ntimeout_ms = 15000\nrefresh_interval_ms = 0\n", providerID, strconv.Quote(helper), strconv.Quote(profile.ID))
	}
	result := []byte(content + section + managedEnd + "\n")
	if _, err = parseConfig(result); err != nil {
		return nil, err
	}
	return result, nil
}
func restoreConfig(original []byte, b baseline) ([]byte, error) {
	content, err := removeManaged(string(original))
	if err != nil {
		return nil, err
	}
	content, err = setRoot(content, "model", b.Model)
	if err != nil {
		return nil, err
	}
	content, err = setRoot(content, "model_provider", b.Provider)
	if err != nil {
		return nil, err
	}
	return []byte(content), nil
}
func commitConfig(original, next []byte) error {
	latest, err := readConfig()
	if err != nil {
		return err
	}
	if !bytes.Equal(latest, original) {
		return errors.New("Codex 配置刚刚被其他程序修改，请刷新后重试")
	}
	dir := filepath.Join(filepath.Dir(dataPath()), "backups")
	backup := filepath.Join(dir, time.Now().Format("20060102-150405.000000000")+".toml")
	if err = atomicWrite(backup, original); err != nil {
		return fmt.Errorf("备份失败，未修改配置: %w", err)
	}
	return atomicWrite(configPath(), next)
}
