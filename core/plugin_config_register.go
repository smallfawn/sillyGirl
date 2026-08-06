package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/goccy/go-json"
	"github.com/smallfawn/sillyGirl/utils"
)

func registerNodePluginConfigSchema(path, uuid string) error {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(uuid) == "" {
		return errors.New("插件路径或 UUID 为空")
	}
	bin, err := resolveNodeCommand()
	if err != nil {
		return err
	}
	workDir := nodePluginWorkDir(path)
	preload, err := ensureNodeRuntimePreload()
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp("", "sillygirl-plugin-schema-*.json")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	temp.Close()
	defer os.Remove(tempPath)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--require", preload, path)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		"PLUGIN_ID="+uuid,
		"PLUGIN_CONFIG_JSON="+string(utils.JsonMarshal(getPluginUserConfig(uuid))),
		"SILLYGIRL_CONFIG_REGISTER_ONLY=true",
		"SILLYGIRL_CONFIG_SCHEMA_FILE="+tempPath,
	)
	cmd.Env = append(cmd.Env, pluginFormRegistrationEnv(path)...)
	cmd.Env = append(cmd.Env, sillyGirlRuntimeEnv()...)
	if nodePath := nodeRuntimeNodePath(); nodePath != "" {
		cmd.Env = append(cmd.Env, "NODE_PATH="+nodePath)
	}
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return fmt.Errorf("配置注册超时：%v", ctx.Err())
	}
	if err != nil {
		return fmt.Errorf("配置注册脚本执行失败：%v：%s", err, strings.TrimSpace(string(output)))
	}
	data, err := os.ReadFile(tempPath)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return errors.New("插件没有导出配置 schema")
	}
	if err := saveRegisteredPluginForms(uuid, data); err != nil {
		return err
	}
	console.Log("已注册插件表单 %s (%s)", filepath.Base(path), uuid)
	return nil
}

type registeredPluginForms struct {
	Plugin map[string]interface{}    `json:"plugin"`
	User   *pluginUserFormDefinition `json:"user"`
}

func pluginFormRegistrationEnv(path string) []string {
	data, _ := os.ReadFile(path)
	source := string(data)
	return []string{
		fmt.Sprintf("SILLYGIRL_EXPECT_PLUGIN_FORM=%t", pluginFormCallPattern.MatchString(source)),
		fmt.Sprintf("SILLYGIRL_EXPECT_USER_FORM=%t", userFormCallPattern.MatchString(source)),
	}
}

func saveRegisteredPluginForms(uuid string, data []byte) error {
	forms := registeredPluginForms{}
	if err := json.Unmarshal(data, &forms); err != nil {
		return fmt.Errorf("表单 schema 解析失败：%v", err)
	}
	if len(forms.Plugin) == 0 && forms.User == nil {
		return errors.New("插件没有导出 plugin.Form 或 user.Form")
	}
	if len(forms.Plugin) != 0 {
		if _, _, err := SetBucketKeyValue(pluginConfigSchemas, uuid, forms.Plugin); err != nil {
			return err
		}
	} else {
		_, _, _ = SetBucketKeyValue2(pluginConfigSchemas, uuid, nil)
	}
	if forms.User != nil {
		if err := validateUserFormDefinition(*forms.User); err != nil {
			return err
		}
		if _, _, err := SetBucketKeyValue(pluginUserFormSchemas, uuid, forms.User); err != nil {
			return err
		}
	} else {
		_, _, _ = SetBucketKeyValue2(pluginUserFormSchemas, uuid, nil)
	}
	return nil
}
