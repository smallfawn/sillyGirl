package core

import (
	"bufio"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	cron "github.com/robfig/cron/v3"
	"github.com/smallfawn/sillyGirl/core/common"
	"github.com/smallfawn/sillyGirl/core/storage"
	"github.com/smallfawn/sillyGirl/utils"
)

func init() {
	go initNodePlugins()
}

var processes sync.Map
var nodePluginSourceHashCache sync.Map

func initNodePlugins() {
	root := strings.ReplaceAll(nodePluginsRoot(), "\\", "/")
	plugins := []string{root}
	os.Mkdir(root, 0755)
	_ = ensureNodeSillygirlModule(root)
	_, _ = ensurePythonSillygirlModule()
	// fmt.Println("root", root)
	files, _ := os.ReadDir(root)
	for _, file := range files {
		if shouldIgnoreNodePluginEntry(file.Name()) {
			continue
		}
		path := root + "/" + file.Name()
		if !file.IsDir() {
			if class, ok := CheckMainIndex(file.Name()); ok {
				AddNodePlugin(path, nodePluginNameFromPath(path), class)
			}
			continue
		}
		plugins = append(plugins, path)
		index, class := FindMainIndex(path)

		if index != "" {
			AddNodePlugin(index, nodePluginNameFromPath(index), class)
		}
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Println("创建监视器失败：", err)
		return
	}
	defer watcher.Close()
	// 要监控的文件夹路径
	for _, dir := range plugins {
		err = watcher.Add(dir)
		if err != nil {
			fmt.Println("添加监视目录失败：", err)
			return
		}
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			// fmt.Println(event.Name, "op", event.Op.String())
			event.Name = strings.ReplaceAll(event.Name, "\\", "/")
			files := strings.Split(strings.Replace(event.Name, root+"/", "", 1), "/")
			var plugin_dir = false
			var plugin_index = false
			var plugin_name = ""
			var class = ""
			switch len(files) {
			case 1:
				if shouldIgnoreNodePluginEntry(files[0]) {
					continue
				}
				if class, plugin_index = CheckMainIndex(files[0]); plugin_index {
					plugin_name = strings.TrimSuffix(files[0], filepath.Ext(files[0]))
				} else {
					plugin_dir = true
					// fmt.Println("目录事件")
					plugin_name = files[0]
				}
			case 2:
				class, plugin_index = CheckMainIndex(files[1])
				plugin_name = files[0]
			}
			if plugin_name == "." {
				continue
			}
			switch event.Op.String() {
			case "CREATE":
				if plugin_dir {
					info, err := os.Stat(event.Name)
					// fmt.Println(err)
					if err == nil && info.IsDir() {
						if shouldIgnoreNodePluginEntry(filepath.Base(event.Name)) {
							continue
						}
						index, class := FindMainIndex(event.Name)
						if class != "" {
							AddNodePlugin(index, nodePluginNameFromPath(index), class)
						}
						watcher.Add(event.Name)
						// fmt.Println("增加插件目录", event.Name)
					} else {
						// fmt.Println("非插件目录", event.Name)
					}
				} else if plugin_index {
					// fmt.Println("增加插件", event.Name)
					// RemNodePlugin(plugin_name)
					AddNodePlugin(event.Name, plugin_name, class)
				}
			case "REMOVE", "RENAME", "REMOVE|RENAME", "REMOVE|WRITE":
				if plugin_dir {
					watcher.Remove(event.Name)
					// fmt.Println("移除插件目录", event.Name)
					// fmt.Println("移除插件", plugin_name)
					AddNodePlugin(event.Name, plugin_name, UNKNOWN)
				} else if plugin_index {
					// fmt.Println("移除插件", plugin_name)
					AddNodePlugin(event.Name, plugin_name, class)
				}
			case "WRITE": //, "CHMOD"
				if plugin_index {
					AddNodePlugin(event.Name, plugin_name, class)
					// fmt.Println("变更插件", event.Name, plugin_name)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			fmt.Println("错误：", err)
		}
	}
}

func nameUuid(name string) string {
	hash := sha1.Sum([]byte(name))
	return strings.ReplaceAll(uuid.NewSHA1(uuid.Nil, hash[:]).String(), "-", "_")
}

func isNameUuid(uuid string) bool {
	return strings.Contains(uuid, "_")
}

func AddNodePlugin(path, name, class string) error {
	pluginLock.Lock()
	defer pluginLock.Unlock()

	return addNodePluginLocked(path, name, class)
}

func addNodePluginLocked(path, name, class string) error {
	if name == "" {
		return nil
	}
	uuid := nameUuid(name)
	cleanPath := filepath.Clean(path)
	file, err := os.Open(path)
	if err != nil {
		rf := unloadNodePluginLocked(uuid)
		nodePluginSourceHashCache.Delete(cleanPath)
		if rf != nil {
			console.Log("已卸载 %s%s", rf.Title, rf.Suffix)
		}
		return err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	script := string(data)
	hash := fmt.Sprintf("%x", sha1.Sum(data))
	if loaded := loadedNodePluginLocked(uuid); loaded != nil && samePath(loaded.Path, cleanPath) {
		if cached, ok := nodePluginSourceHashCache.Load(cleanPath); ok && cached == hash {
			return nil
		}
	}
	//移除
	if script == "" {
		unloadNodePluginLocked(uuid)
		nodePluginSourceHashCache.Delete(cleanPath)
		return nil
	}
	rf := unloadNodePluginLocked(uuid)
	// plugins_id.Store(uuid, path)
	// fmt.Println("add,", uuid, name)
	f, cbs := pluginParse(script, uuid)
	f.Reload = func() { //重载
		AddNodePlugin(path, name, class)
	}

	f.Type = class
	switch f.Type {
	case NODE:
		f.Suffix = ".js"
	case PYTHON:
		f.Suffix = ".py"
	}
	f.Path = path
	if f.HasForm {
		var err error
		switch class {
		case NODE:
			err = registerNodePluginConfigSchema(path, uuid)
		case PYTHON:
			err = registerPythonPluginConfigSchema(path, uuid)
		}
		if err != nil {
			console.Warn("插件配置自动注册失败 %s: %v", name, err)
		}
	}
	f.Handle = func(s common.Sender) interface{} {
		console := &Console{UUID: uuid}
		s.SetPluginID(uuid)
		plt := s.GetImType()
		bin := ""
		var cmd *exec.Cmd
		workDir := filepath.Dir(path)
		switch class {
		case NODE:
			workDir = nodePluginWorkDir(path)
			if err := ensureNodeSillygirlModule(workDir); err != nil {
				console.Error("NodeJS sillygirl 模块初始化失败：%v", err)
				return nil
			}
			if err := ensureNodeRuntimeDependencies(workDir); err != nil {
				console.Error("NodeJS sillygirl 运行时依赖安装失败：%v", err)
				return nil
			}
			var err error
			bin, err = resolveNodeCommand()
			if err != nil {
				console.Error("NodeJS 运行时未找到：%v", err)
				return nil
			}
			if preload, err := ensureNodeRuntimePreload(); err == nil {
				cmd = exec.Command(bin, "--require", preload, path)
			} else {
				console.Error("NodeJS 运行时预加载失败：%v", err)
				cmd = exec.Command(bin, path)
			}
		case PYTHON:
			var args []string
			var err error
			bin, args, err = resolvePythonCommand()
			if err != nil {
				console.Error("Python 运行时未找到：%v", err)
				return nil
			}
			args = append(args, "-u", path)
			cmd = exec.Command(bin, args...)
			pythonPath, err := ensurePythonSillygirlModule()
			if err != nil {
				console.Error("Python sillygirl 模块初始化失败：%v", err)
				return nil
			}
			if err := ensurePipxRuntimeEnv(); err != nil {
				console.Error("Python sillygirl 运行时依赖安装失败：%v", err)
				return nil
			}
			cmd.Env = append(cmd.Env, pythonRuntimeEnvVars(pythonPath)...)
		}

		cmd.Dir = workDir
		RUNTIME_ID := utils.GenUUID()
		cmd.Env = append(os.Environ(), cmd.Env...)
		if class == NODE {
			if nodePath := nodeRuntimeNodePath(); nodePath != "" {
				cmd.Env = append(cmd.Env, "NODE_PATH="+nodePath)
			}
		}
		grpcAddress, grpcErr := grpcClientAddress()
		if grpcErr != nil {
			console.Error("gRPC 插件运行时未就绪：%v", grpcErr)
			return nil
		}
		cmd.Env = append(cmd.Env, "RUNTIME_ID="+RUNTIME_ID)
		cmd.Env = append(cmd.Env, "PLUGIN_ID="+uuid)
		cmd.Env = append(cmd.Env, "SILLYGIRL_GRPC_ADDR="+grpcAddress)
		cmd.Env = append(cmd.Env, "SILLYGIRL_GRPC_TOKEN="+grpcRuntimeMetadataToken())
		cmd.Env = append(cmd.Env, sillyGirlRuntimeEnv()...)
		if class == NODE || class == PYTHON {
			cmd.Env = append(cmd.Env, "PLUGIN_CONFIG_JSON="+string(utils.JsonMarshal(getPluginUserConfig(uuid))))
		}
		if class == NODE {
			if f.Web {
				cmd.Env = append(cmd.Env, "SILLYGIRL_WEB=true")
			}
		}
		// 获取标准输出和标准错误输出的管道
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			console.Error("获取插件标准输出管道失败：%v", err)
			return nil
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			_ = stdout.Close()
			console.Error("获取插件标准错误输出管道失败：%v", err)
			return nil
		}
		var wg sync.WaitGroup
		wg.Add(2)
		// 处理标准输出
		go func() {
			defer wg.Done()

			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				data := scanner.Text()
				fmt.Println(data)

				// if _, err := file.WriteString(data + "\n"); err != nil {
				// 	fmt.Printf("写入文件失败：%v\n", err)
				// }
			}
		}()
		// 处理标准错误输出
		go func() {
			defer wg.Done()
			scanner := bufio.NewScanner(stderr)
			if f.OnStart {
				for scanner.Scan() {
					fmt.Println(scanner.Text())
				}
			} else {
				lines := []string{}
				for scanner.Scan() {
					data := scanner.Text()
					lines = append(lines, data)
				}
				if len(lines) != 0 {
					console.Error(strings.Join(lines, "\n"))
				}

			}
		}()
		registerRuntimePlugin(RUNTIME_ID, uuid)
		register := createSenderRegister(RUNTIME_ID)
		if (plt) != "*" {
			cmd.Env = append(cmd.Env, "SENDER_ID="+register(s))
			if err := cmd.Start(); err != nil {
				_ = stdout.Close()
				_ = stderr.Close()
				wg.Wait()
				deleteSenderRegister(RUNTIME_ID)
				console.Error("插件进程启动失败：%v", err)
				return nil
			}
			processes.Store(cmd, s)
			defer deleteSenderRegister(RUNTIME_ID)
			defer processes.Delete(cmd)
			if err := cmd.Wait(); err != nil {
				console.Error("插件进程执行失败：%v", err)
				wg.Wait()
				return nil
			}
			wg.Wait()
		} else {
			if err := cmd.Start(); err != nil {
				_ = stdout.Close()
				_ = stderr.Close()
				wg.Wait()
				deleteSenderRegister(RUNTIME_ID)
				console.Error("插件进程启动失败：%v", err)
				return nil
			}
			processes.Store(cmd, s)
			processes.Range(func(key, value any) bool {
				p := key.(*exec.Cmd)
				if p == cmd {
					return true
				}
				s := value.(common.Sender)
				if s.GetPluginID() == uuid {
					func() {
						defer func() {
							recover()
						}()
						if p.Process.Kill() == nil {
							processes.Delete(key)
						}
					}()
				}
				return true
			})
			go func() {
				defer deleteSenderRegister(RUNTIME_ID)
				defer processes.Delete(cmd)
				if err := cmd.Wait(); err != nil {
					console.Error("插件后台进程执行失败：%v", err)
				}
				wg.Wait()
			}()
		}
		return nil
	}
	for _, cb := range cbs {
		cb()
	}
	if !f.Disable { //!f.OnStart &&
		if rf == nil {
			// console.Log("已加载 %s%s", f.Title, f.Suffix)
		} else {
			console.Log("已重载 %s%s", f.Title, f.Suffix)
		}
	}
	AddCommand([]*common.Function{f})
	nodePluginSourceHashCache.Store(cleanPath, hash)
	return nil
}

func loadedNodePluginLocked(uuid string) *common.Function {
	for _, f := range Functions {
		if f != nil && f.UUID == uuid {
			return f
		}
	}
	return nil
}

func unloadNodePluginLocked(uuid string) *common.Function {
	for i := range Functions {
		if Functions[i].UUID != uuid {
			continue
		}
		rf := Functions[i]
		DestroyAdapterByUUID(uuid)
		rf.Running = false
		if len(rf.CronIds) != 0 {
			for _, id := range rf.CronIds {
				CRON.Remove(cron.EntryID(id))
			}
		}
		Functions = append(Functions[:i], Functions[i+1:]...)
		CancelPluginCrons(uuid)
		CancelPluginWebs(uuid)
		CancelPluginlistening(uuid)
		remStatic(uuid)
		storage.DisableHandle(uuid)
		return rf
	}
	return nil
}

var typeat = `declare class Sender {
	private uuid;
	private destoried;
	constructor(uuid: string);
	destroy(): void;
	getUserId(): Promise<string>;
	getUserName(): Promise<string>;
	getChatId(): Promise<string>;
	getChatName(): Promise<string>;
	getMessageId(): Promise<string>;
	getPlatform(): Promise<string>;
	getBotId(): Promise<string>;
	getContent(): Promise<string>;
	isAdmin(): Promise<boolean>;
	param(key: number | string): Promise<string>;
	setContent(content: string): Promise<undefined>;
	continue(): Promise<undefined>;
	getAdapter(): Promise<Adapter>;
	listen(options?: {
			rules?: string[];
			timeout?: number;
			handle?: (s: Sender) => Promise<string | void> | string | void;
			listen_private?: boolean;
			listen_group?: boolean;
			allow_platforms?: string[];
			prohibit_platforms?: string[];
			allow_groups?: string[];
			prohibit_groups?: string[];
			allow_users?: string[];
			prohibit_users?: string[];
	}): Promise<Sender | undefined>;
	holdOn(str: string): string;
	reply(content: string): Promise<string>;
	doAction(options: Record<string, any>): Promise<any>;
	getEvent(): Promise<Record<string, any>>;
}
declare class Bucket {
	private name;
	constructor(name: string);
	transform(v: string | undefined): string | number | boolean | undefined;
	reverseTransform(value: any): string;
	get(key: string, defaultValue?: any): Promise<any>;
	set(key: string, value: any): Promise<{
			message?: string;
			changed?: boolean;
	}>;
	getAll(): Promise<Record<string, any>>;
	delete(key: string): Promise<{
			message?: string;
			changed?: boolean;
	}>;
	deleteAll(): Promise<undefined>;
	keys(): Promise<string[]>;
	len(): Promise<number>;
	buckets(): Promise<string[]>;
	watch(key: string, handle: (old: any, now: any, key: string) => StorageModifier | void): void;
	getName(): Promise<string>;
}
interface SillyGirlUserBindings {
	qq: string;
	telegram: string;
	smallcat_openids: string[];
}
interface SillyGirlUser {
	id: string;
	username: string;
	nickname: string;
	disabled: boolean;
	authorized: boolean;
	bindings: SillyGirlUserBindings;
}
declare function userList(): Promise<SillyGirlUser[]>;
declare class QingLong {
	id: number;
	uuid: string;
	name: string;
	address: string;
	constructor(options: { id: number | string });
	request(method: string, path: string, body?: any, query?: Record<string, any>): Promise<any>;
	getEnvs(options?: Record<string, any> | string): Promise<any>;
	getEnvById(id: number | string): Promise<any>;
	createEnv(env: any): Promise<any>;
	updateEnv(env: any): Promise<any>;
	deleteEnvs(ids: any): Promise<any>;
	moveEnv(id: number | string, fromIndex: number, toIndex: number): Promise<any>;
	moveEnv(id: number | string, body: Record<string, any>): Promise<any>;
	disableEnvs(ids: any): Promise<any>;
	enableEnvs(ids: any): Promise<any>;
	updateEnvNames(ids: any, name: string): Promise<any>;
	updateEnvNames(body: Record<string, any>): Promise<any>;
	systemNotify(title: string, content: string): Promise<any>;
}
declare class SmallCat {
	id: number;
	uuid: string;
	name: string;
	address: string;
	constructor(options: { id: number | string });
	request(method: string, path: string, body?: any, query?: Record<string, any>): Promise<any>;
	createQr(type: any): Promise<any>;
	checkQr(uuid: string): Promise<any>;
	addUser(options: Record<string, any>): Promise<any>;
	rescanUser(options: Record<string, any>): Promise<any>;
	authorizedUsers(): Promise<any>;
	userList(): Promise<any>;
	checkUsers(options: Record<string, any>): Promise<any>;
	setUserRemark(options: Record<string, any>): Promise<any>;
	setUserDisabled(options: Record<string, any>): Promise<any>;
	deleteUser(options: Record<string, any>): Promise<any>;
	proxyList(): Promise<any>;
	testProxy(options: Record<string, any>): Promise<any>;
	addProxy(options: Record<string, any>): Promise<any>;
	deleteProxy(options: Record<string, any>): Promise<any>;
	creditBalance(): Promise<any>;
	creditLedger(query?: Record<string, any> | number): Promise<any>;
	getCode(options: { openid: string; appid: string }): Promise<any>;
	getSession(options: { openid: string; appid: string }): Promise<any>;
	refreshSession(options: { openid: string; appid: string }): Promise<any>;
	getUserInfo(options: { openid: string; appid: string }): Promise<any>;
	getEncryptKey(options: { openid: string; appid: string }): Promise<any>;
	getPhoneNumber(options: { openid: string; appid: string }): Promise<any>;
	cloud(options: Record<string, any>): Promise<any>;
	gateway(options: Record<string, any>): Promise<any>;
	qrCodeAuth(options: Record<string, any>): Promise<any>;
	oAuth(options: Record<string, any>): Promise<any>;
	translateLink(options: Record<string, any>): Promise<any>;
	autoAuth(options: Record<string, any>): Promise<any>;
	appMsgExt(options: Record<string, any>): Promise<any>;
	appMsgLike(options: Record<string, any>): Promise<any>;
}
declare class DaiDai {
	id: number;
	uuid: string;
	name: string;
	address: string;
	constructor(options: { id: number | string });
	request(method: string, path: string, body?: any, query?: Record<string, any>): Promise<any>;
	getEnvs(options?: Record<string, any> | string): Promise<any>;
	getEnvById(id: number | string): Promise<any>;
	createEnv(env: any): Promise<any>;
	updateEnv(env: any): Promise<any>;
	deleteEnv(id: number | string): Promise<any>;
	deleteEnvs(ids: any): Promise<any>;
	enableEnv(id: number | string): Promise<any>;
	disableEnv(id: number | string): Promise<any>;
	enableEnvs(ids: any): Promise<any>;
	disableEnvs(ids: any): Promise<any>;
	getTasks(options?: Record<string, any> | string): Promise<any>;
	getTaskById(id: number | string): Promise<any>;
	createTask(task: any): Promise<any>;
	updateTask(task: any): Promise<any>;
	deleteTask(id: number | string): Promise<any>;
	runTask(id: number | string): Promise<any>;
	stopTask(id: number | string): Promise<any>;
	enableTask(id: number | string): Promise<any>;
	disableTask(id: number | string): Promise<any>;
	systemNotify(title: string, content: string): Promise<any>;
}
interface SillyGirlSchemaNode {
	schema: Record<string, any>;
	setTitle(value: string): SillyGirlSchemaNode;
	setDescription(value: string): SillyGirlSchemaNode;
	setDefault(value: any): SillyGirlSchemaNode;
	setEnum(value: any[]): SillyGirlSchemaNode;
	setEnumNames(value: string[]): SillyGirlSchemaNode;
	setRequired(value: string[] | boolean): SillyGirlSchemaNode;
	setFormat(value: string): SillyGirlSchemaNode;
	setMin(value: number): SillyGirlSchemaNode;
	setMax(value: number): SillyGirlSchemaNode;
	setMinLength(value: number): SillyGirlSchemaNode;
	setMaxLength(value: number): SillyGirlSchemaNode;
	setPattern(value: string): SillyGirlSchemaNode;
	setWidget(value: string): SillyGirlSchemaNode;
	toJSON(): Record<string, any>;
}
declare const sillyGirlCreateSchema: {
	string(): SillyGirlSchemaNode;
	number(): SillyGirlSchemaNode;
	integer(): SillyGirlSchemaNode;
	boolean(): SillyGirlSchemaNode;
	array(item?: any): SillyGirlSchemaNode;
	object(props?: Record<string, any>): SillyGirlSchemaNode;
};
declare class SillyGirlPluginConfig {
	uuid: string;
	jsonSchema: Record<string, any>;
	userConfig: Record<string, any>;
	ready: Promise<Record<string, any>>;
	constructor(schema: any);
	get(): Promise<Record<string, any>>;
	Get(): Promise<Record<string, any>>;
	set(values?: Record<string, any>): Promise<{ error: string }>;
	Set(values?: Record<string, any>): Promise<{ error: string }>;
}
declare function form(schema: any): SillyGirlPluginConfig;
declare function pluginConfigDefaults(schema: any): any;
interface StorageModifier {
	echo?: string;
	now?: any;
	message?: string;
	error?: string;
}
interface Message {
	message_id?: string;
	user_id: string;
	chat_id?: string;
	content: string;
	user_name?: string;
	chat_name?: string;
}
interface PushAdminOptions {
	platform?: string | string[];
	platforms?: string[];
	botId?: string;
	bot_id?: string;
	userIds?: string[];
	users?: string[];
}
declare class Adapter {
	platform: string;
	bot_id: string;
	call: any;
	constructor(options: {
			platform: string;
			bot_id: string;
			replyHandler?: (message: Message) => Promise<string | undefined>;
			actionHandler?: (message: Message) => Promise<string | undefined>;
	});
	receive(message: Message): Promise<undefined>;
	push(message: Message): Promise<string>;
	destroy(): Promise<void>;
	sender(options: any): Promise<Sender>;
}
declare let sender: Sender;
declare function pushAdmin(content: string, options?: PushAdminOptions): Promise<{
	platform: string;
	bot_id: string;
	user_id: string;
	message_id?: string;
	error?: string;
}[]>;
declare function sleep(ms?: number): Promise<unknown>;
interface UpdateOptions {
	releaseRepo?: string;
	releaseTag?: string;
	releaseAsset?: string;
	executablePath?: string;
	timeout?: number;
	restart?: boolean;
}
interface UpdateResult {
	mode?: string;
	repo: string;
	before: string;
	after: string;
	changed: boolean;
	output: string;
	restarted: boolean;
}
declare function restart(): Promise<{ message?: string; changed?: boolean }>;
declare function update(options?: UpdateOptions): Promise<UpdateResult>;
interface CQItem {
	type: string;
	params: Record<string, string>;
}
interface CQParams {
	[key: string]: string | number | boolean;
}
declare let utils: {
	buildCQTag: (type: string, params: CQParams, prefix?: string) => string;
	parseCQText: (text: string, prefix?: string) => (string | CQItem)[];
	image: (url: string) => string;
	video: (url: string) => string;
};
declare let console: {
	log(...args: any[]): void;
	info(...args: any[]): void;
	error(...args: any[]): void;
	debug(...args: any[]): void;
};
export { Adapter, Bucket, QingLong, SmallCat, DaiDai, SillyGirlUser, SillyGirlUserBindings, userList, sillyGirlCreateSchema, SillyGirlPluginConfig, form, pluginConfigDefaults, sender, pushAdmin, sleep, restart, update, utils, console };
`

func defaultScript(title string) string {
	return `/**
* @title ` + title + `
* @desc 🐒这个人很懒什么都没有留下
* @author ` + sillyGirl.GetString("author", "佚名") + `
* @version v1.0.0
*/

const {
  sender: s,
  Bucket,
  QingLong,
  SmallCat,
  DaiDai,
  userList,
  sillyGirlCreateSchema,
  SillyGirlPluginConfig,
  form,
  pushAdmin,
  restart,
  update,
  utils: { buildCQTag, image, video },
} = require("sillygirl");
`
}

func defaultPythonScript(title string) string {
	return `"""
* @title ` + title + `
* @desc 这个人很懒什么都没有留下
* @author ` + sillyGirl.GetString("author", "佚名") + `
* @version v1.0.0
"""

import asyncio
from sillygirl import sender as s


async def main():
    await s.reply("pong")


asyncio.run(main())
`
}

const (
	NODE    = "node"
	PYTHON  = "python"
	UNKNOWN = "unknown"
)

func FindMainIndex(home string) (string, string) {
	if info, err := os.Stat(home); err == nil && !info.IsDir() {
		switch {
		case strings.EqualFold(filepath.Ext(home), ".js") && filepath.Base(home) != "demo.main.js":
			return strings.ReplaceAll(home, "\\", "/"), NODE
		case strings.EqualFold(filepath.Ext(home), ".py"):
			return strings.ReplaceAll(home, "\\", "/"), PYTHON
		}
	}
	if info, err := os.Stat(home + "/main.js"); err == nil && !info.IsDir() {
		return home + "/main.js", NODE
	}
	if info, err := os.Stat(home + "/main.py"); err == nil && !info.IsDir() {
		return home + "/main.py", PYTHON
	}
	pluginName := filepath.Base(filepath.Clean(home))
	if pluginName != "." && pluginName != string(filepath.Separator) {
		index := filepath.Join(home, pluginName+".js")
		if info, err := os.Stat(index); err == nil && !info.IsDir() {
			return strings.ReplaceAll(index, "\\", "/"), NODE
		}
		index = filepath.Join(home, pluginName+".py")
		if info, err := os.Stat(index); err == nil && !info.IsDir() {
			return strings.ReplaceAll(index, "\\", "/"), PYTHON
		}
	}
	files, err := os.ReadDir(home)
	if err == nil {
		indexes := []string{}
		classes := map[string]string{}
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			class, ok := CheckMainIndex(file.Name())
			if !ok {
				continue
			}
			index := filepath.Join(home, file.Name())
			indexes = append(indexes, index)
			classes[index] = class
		}
		if len(indexes) == 1 {
			index := strings.ReplaceAll(indexes[0], "\\", "/")
			return index, classes[indexes[0]]
		}
	}
	return "", ""
}

func CheckMainIndex(filename string) (string, bool) {
	switch filename {
	case "main.js":
		return NODE, true
	case "main.py":
		return PYTHON, true
	}
	if strings.EqualFold(filepath.Ext(filename), ".js") && filename != "demo.main.js" {
		return NODE, true
	}
	if strings.EqualFold(filepath.Ext(filename), ".py") {
		return PYTHON, true
	}
	return "", false
}

func sillyGirlRuntimeEnv() []string {
	appDir := detectSillyGirlAppDir()
	latest, source := latestAppVersion()
	values := []string{
		"SILLYGIRL_VERSION=" + currentAppVersion(),
		"SILLYGIRL_REMOTE_VERSION=" + latest,
		"SILLYGIRL_VERSION_SOURCE=" + source,
		"SILLYGIRL_REPOSITORY=" + appRepository,
		"SILLYGIRL_EXEC_PATH=" + currentExecutablePath(),
	}
	if proxy := strings.TrimSpace(githubAcceleratorPrefix()); proxy != "" {
		values = append(values, "SILLYGIRL_GITHUB_PROXY="+proxy)
	}
	if appDir != "" {
		values = append(values, "SILLYGIRL_APP_DIR="+appDir)
	}
	return values
}

func currentExecutablePath() string {
	execPath, err := os.Executable()
	if err != nil || strings.TrimSpace(execPath) == "" {
		execPath, _ = filepath.Abs(os.Args[0])
	}
	return execPath
}

func detectSillyGirlAppDir() string {
	wd, _ := os.Getwd()
	candidates := dedupeCleanPaths([]string{
		os.Getenv("SILLYGIRL_APP_DIR"),
		wd,
		utils.ExecPath,
		filepath.Dir(utils.ExecPath),
		"/app",
		"/data/sillyGirl",
	})
	for _, candidate := range candidates {
		if repo := gitTopLevel(candidate); repo != "" && looksLikeSillyGirlRepo(repo) {
			return repo
		}
		if looksLikeSillyGirlRepo(candidate) {
			return candidate
		}
	}
	if wd != "" {
		return wd
	}
	return ""
}

func gitTopLevel(dir string) string {
	if strings.TrimSpace(dir) == "" {
		return ""
	}
	output, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func looksLikeSillyGirlRepo(dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	required := []string{
		filepath.Join(dir, "go.mod"),
		filepath.Join(dir, "core", "version.go"),
		filepath.Join(dir, "proto3", "sillygirl.js"),
	}
	for _, item := range required {
		if _, err := os.Stat(item); err != nil {
			return false
		}
	}
	return true
}
