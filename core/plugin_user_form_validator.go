package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfawn/sillyGirl/core/common"
	"github.com/smallfawn/sillyGirl/utils"
)

const nodeUserFormValidatorScript = `
const fs=require("fs"); globalThis.require=require; const input=JSON.parse(fs.readFileSync(process.argv[2],"utf8"));
(async()=>{const errors=[],failed=new Set();for(const item of input.validators){if(failed.has(item.field))continue;try{
  const fn=(0,eval)("("+item.source+")");
  if(typeof fn!=="function") throw new Error("test source is not a function");
  const result=await fn(input.values[item.field],input.context);
  if(result===true) continue;
  const message=typeof result==="string"&&result.trim()?result:(item.message||"验证未通过");
  errors.push({field:item.field,code:"test",message});failed.add(item.field);
}catch(error){console.error("validator failed",item.field,error&&error.message||error);errors.push({field:item.field,code:"test",message:"远程验证异常"});failed.add(item.field);}}
fs.writeFileSync(process.argv[3],JSON.stringify(errors));})().catch(error=>{console.error(error);process.exit(1)});
`

const pythonUserFormValidatorScript = `
import ast, asyncio, inspect, json, sys, textwrap

data=json.load(open(sys.argv[1],encoding="utf-8"))
errors=[]
failed=set()
async def main():
    for item in data["validators"]:
        if item["field"] in failed: continue
        try:
            source=textwrap.dedent(item["source"]).strip()
            scope={}
            if source.startswith("lambda"):
                fn=eval(source,scope)
            else:
                tree=ast.parse(source)
                names=[node.name for node in tree.body if isinstance(node,(ast.FunctionDef,ast.AsyncFunctionDef))]
                exec(compile(tree,"<user-form-test>","exec"),scope)
                fn=scope.get(names[-1]) if names else None
            if not callable(fn): raise RuntimeError("test source is not a function")
            result=fn(data["values"].get(item["field"]),data["context"])
            if inspect.isawaitable(result): result=await result
            if result is True: continue
            message=result.strip() if isinstance(result,str) and result.strip() else (item.get("message") or "验证未通过")
            errors.append({"field":item["field"],"code":"test","message":message})
            failed.add(item["field"])
        except Exception as exc:
            print("validator failed",item["field"],str(exc),file=sys.stderr)
            errors.append({"field":item["field"],"code":"test","message":"远程验证异常"})
            failed.add(item["field"])
    json.dump(errors,open(sys.argv[2],"w",encoding="utf-8"),ensure_ascii=False,separators=(",",":"))
asyncio.run(main())
`

var userFormValidatorSlots = make(chan struct{}, 4)

type userFormValidatorInput struct {
	Values     map[string]interface{}   `json:"values"`
	Context    map[string]interface{}   `json:"context"`
	Validators []userFormValidatorEntry `json:"validators"`
}

type userFormValidatorEntry struct {
	Field   string `json:"field"`
	Source  string `json:"source"`
	Message string `json:"message,omitempty"`
}

func runPluginUserFormValidators(parent context.Context, plugin *common.Function, definition pluginUserFormDefinition, user *normalUser, values map[string]interface{}) []gin.H {
	if plugin == nil || user == nil || len(definition.Validators) == 0 {
		return nil
	}
	entries := []userFormValidatorEntry{}
	fields := make([]string, 0, len(definition.Validators))
	for field := range definition.Validators {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	for _, field := range fields {
		value, exists := values[field]
		if !exists || value == nil || fmt.Sprintf("%v", value) == "" {
			continue
		}
		validators := definition.Validators[field]
		for _, validator := range validators {
			if validator.Runtime != plugin.Type {
				return []gin.H{{"field": field, "code": "test", "message": "校验器运行时配置错误"}}
			}
			entries = append(entries, userFormValidatorEntry{Field: field, Source: validator.Source, Message: validator.Message})
		}
	}
	if len(entries) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	select {
	case userFormValidatorSlots <- struct{}{}:
		defer func() { <-userFormValidatorSlots }()
	case <-ctx.Done():
		return []gin.H{{"field": "", "code": "test", "message": "远程验证等待超时"}}
	}
	bindings := loadNormalUserBindings(user.Username)
	payload := userFormValidatorInput{
		Values: values,
		Context: map[string]interface{}{
			"values": values,
			"user":   map[string]interface{}{"id": user.ID, "username": user.Username, "nickname": user.Nickname, "bindings": bindings},
			"plugin": map[string]interface{}{"id": plugin.UUID, "title": plugin.Title},
			"config": getPluginUserConfig(plugin.UUID),
		},
		Validators: entries,
	}
	result, err := executeUserFormValidators(ctx, plugin, payload)
	if err != nil {
		console.Warn("插件用户表单验证失败 %s: %v", plugin.UUID, err)
		message := "远程验证执行失败"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			message = "远程验证超时"
		}
		return []gin.H{{"field": "", "code": "test", "message": message}}
	}
	return result
}

func executeUserFormValidators(ctx context.Context, plugin *common.Function, payload userFormValidatorInput) ([]gin.H, error) {
	dir, err := os.MkdirTemp("", "sillygirl-user-form-test-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	inputPath, outputPath := filepath.Join(dir, "input.json"), filepath.Join(dir, "output.json")
	if err = os.WriteFile(inputPath, utils.JsonMarshal(payload), 0600); err != nil {
		return nil, err
	}
	var cmd *exec.Cmd
	switch plugin.Type {
	case NODE:
		bin, resolveErr := resolveNodeCommand()
		if resolveErr != nil {
			return nil, resolveErr
		}
		runner := filepath.Join(dir, "runner.js")
		if err = os.WriteFile(runner, []byte(nodeUserFormValidatorScript), 0600); err != nil {
			return nil, err
		}
		cmd = exec.CommandContext(ctx, bin, runner, inputPath, outputPath)
		cmd.Dir = nodePluginWorkDir(plugin.Path)
		cmd.Env = userFormValidatorEnvironment()
		nodePaths := []string{filepath.Join(cmd.Dir, "node_modules")}
		if nodePath := nodeRuntimeNodePath(); nodePath != "" {
			nodePaths = append(nodePaths, nodePath)
		}
		cmd.Env = append(cmd.Env, "NODE_PATH="+strings.Join(nodePaths, string(os.PathListSeparator)))
	case PYTHON:
		bin, args, resolveErr := resolvePythonCommand()
		if resolveErr != nil {
			return nil, resolveErr
		}
		runner := filepath.Join(dir, "runner.py")
		if err = os.WriteFile(runner, []byte(pythonUserFormValidatorScript), 0600); err != nil {
			return nil, err
		}
		args = append(args, "-u", runner, inputPath, outputPath)
		cmd = exec.CommandContext(ctx, bin, args...)
		cmd.Dir = filepath.Dir(plugin.Path)
		if pythonPath, pathErr := ensurePythonSillygirlModule(); pathErr == nil {
			cmd.Env = append(userFormValidatorEnvironment(), pythonRuntimeEnvVars(pythonPath)...)
		} else {
			return nil, pathErr
		}
	default:
		return nil, fmt.Errorf("不支持的插件运行时 %s", plugin.Type)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%v：%s", err, strings.TrimSpace(string(output)))
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		return nil, err
	}
	if info.Size() > 1<<20 {
		return nil, errors.New("validator output exceeds 1 MiB")
	}
	raw, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, err
	}
	result := []gin.H{}
	if err = json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	if len(result) > len(payload.Validators) {
		return nil, errors.New("validator returned too many errors")
	}
	allowedFields := map[string]bool{}
	for _, item := range payload.Validators {
		allowedFields[item.Field] = true
	}
	for _, item := range result {
		field := strings.TrimSpace(fmt.Sprintf("%v", item["field"]))
		if !allowedFields[field] {
			return nil, errors.New("validator returned an unknown field")
		}
		message := strings.TrimSpace(fmt.Sprintf("%v", item["message"]))
		if len([]rune(message)) > 512 {
			message = string([]rune(message)[:512])
		}
		item["field"], item["code"], item["message"] = field, "test", message
	}
	return result, nil
}

func userFormValidatorEnvironment() []string {
	allowed := map[string]bool{
		"PATH": true, "PATHEXT": true, "SYSTEMROOT": true, "WINDIR": true, "COMSPEC": true,
		"TEMP": true, "TMP": true, "TMPDIR": true, "HOME": true, "USERPROFILE": true,
		"APPDATA": true, "LOCALAPPDATA": true, "LANG": true, "LC_ALL": true,
		"SSL_CERT_FILE": true, "SSL_CERT_DIR": true, "HTTP_PROXY": true, "HTTPS_PROXY": true, "NO_PROXY": true,
	}
	result := []string{}
	for _, item := range os.Environ() {
		key, _, ok := strings.Cut(item, "=")
		if ok && allowed[strings.ToUpper(key)] {
			result = append(result, item)
		}
	}
	return result
}
