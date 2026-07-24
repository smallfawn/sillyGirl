package core

import (
	"bufio"
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

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

func initNodePlugins() {
	root := strings.ReplaceAll(nodePluginsRoot(), "\\", "/")
	plugins := []string{root}
	os.Mkdir(root, 0755)
	_ = ensureNodeSillygirlModule(root)
	// fmt.Println("root", root)
	files, _ := ioutil.ReadDir(root)
	for _, file := range files {
		if shouldIgnoreNodePluginEntry(file.Name()) {
			continue
		}
		path := root + "/" + file.Name()
		if !file.IsDir() {
			if class, ok := CheckMainIndex(file.Name()); ok && class == NODE {
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
				// if files[1] == "main.js" {
				// 	if files[1] == "main.js" {
				// 		plugin_index = true
				// 	}
				// 	// fmt.Println("入口文件事件")
				// }
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
						if class == NODE {
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

	if name == "" {
		return nil
	}
	uuid := nameUuid(name)
	pluginLock.Lock()
	defer pluginLock.Unlock()
	//移除
	var rf *common.Function
	for i := range Functions {
		if Functions[i].UUID == uuid {
			rf = Functions[i]
			DestroyAdapterByUUID(uuid)
			Functions[i].Running = false
			if len(Functions[i].CronIds) != 0 {
				for _, id := range Functions[i].CronIds {
					CRON.Remove(cron.EntryID(id))
				}
			}
			Functions = append(Functions[:i], Functions[i+1:]...)
			CancelPluginCrons(uuid)
			CancelPluginWebs(uuid)
			CancelPluginlistening(uuid)
			remStatic(uuid)
			storage.DisableHandle(uuid)
			break
		}
	}
	file, err := os.Open(path)
	if err != nil {
		if rf != nil {
			console.Log("已卸载 %s%s", rf.Title, rf.Suffix)
		}
		return err
	}
	defer file.Close()
	data, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}
	script := string(data)
	if script == "" {
		return nil
	}
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
			bin = "python3"
			cmd = exec.Command(bin, "-u", path)
			cmd.Env = append(cmd.Env, "PYTHONPATH=/home/user/Code/sillyGirl/proto3")
		}

		cmd.Dir = workDir
		RUNTIME_ID := utils.GenUUID()
		cmd.Env = append(os.Environ(), cmd.Env...)
		if class == NODE {
			if nodePath := nodeRuntimeNodePath(); nodePath != "" {
				cmd.Env = append(cmd.Env, "NODE_PATH="+nodePath)
			}
		}
		cmd.Env = append(cmd.Env, "RUNTIME_ID="+RUNTIME_ID)
		cmd.Env = append(cmd.Env, "PLUGIN_ID="+uuid)
		cmd.Env = append(cmd.Env, "SILLYGIRL_GRPC_ADDR="+grpcClientAddress())
		cmd.Env = append(cmd.Env, "SILLYGIRL_GRPC_TOKEN="+grpcRuntimeMetadataToken())
		if class == NODE {
			cmd.Env = append(cmd.Env, "PLUGIN_CONFIG_JSON="+string(utils.JsonMarshal(getPluginUserConfig(uuid))))
		}
		// 获取标准输出和标准错误输出的管道
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			// fmt.Printf("获取标准输出管道失败：%v\n", err)
			// os.Exit(1)
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			// fmt.Printf("获取标准错误输出管道失败：%v\n", err)
			// os.Exit(1)
		}

		// file, err := os.Create("output.log")
		// if err != nil {
		// 	fmt.Printf("创建文件失败：%v\n", err)
		// 	os.Exit(1)
		// }
		// defer file.Close()
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
		processes.Store(cmd, s)
		register := createSenderRegister(RUNTIME_ID)
		if (plt) != "*" {
			cmd.Env = append(cmd.Env, "SENDER_ID="+register(s))
			err = cmd.Start()
			if err != nil {

			}
			defer deleteSenderRegister(RUNTIME_ID)
			defer processes.Delete(cmd)
			err = cmd.Wait()
			if err != nil {
				fmt.Println("命令执行失败：", err)
				return nil
			}
		} else {
			err = cmd.Start()
			if err != nil {

			}
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
				err = cmd.Wait()
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
declare class qinglong {
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
declare class smallcat {
	id: number;
	uuid: string;
	name: string;
	address: string;
	constructor(options: { id: number | string });
	request(method: string, path: string, body?: any, query?: Record<string, any>): Promise<any>;
	createQr(type: any): Promise<any>;
	checkQr(uuid: string): Promise<any>;
	addUser(options: { code: string; type: number | string; displayName?: string }): Promise<any>;
	userList(): Promise<any>;
	getCode(options: { openid?: string; appid?: string; ref?: string; app_id?: string; target_appid?: string }): Promise<any>;
}
declare class daidai {
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
declare const SillyGirlCreateSchema: {
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
declare function Form(schema: any): SillyGirlPluginConfig;
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
declare function sleep(ms?: number): Promise<unknown>;
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
export { Adapter, Bucket, qinglong, smallcat, daidai, SillyGirlCreateSchema, SillyGirlPluginConfig, Form, pluginConfigDefaults, sender, sleep, utils, console };
`

func defaultScript(title string) string {
	create_at := time.Now().Format("2006-01-02 15:04:05")
	return `/**
* @title ` + title + `
* @create_at ` + create_at + `
* @description 🐒这个人很懒什么都没有留下
* @author ` + sillyGirl.GetString("author", "佚名") + `
* @version v1.0.0
*/

const {
  sender: s,
  Bucket,
  qinglong,
  smallcat,
  daidai,
  utils: { buildCQTag, image, video },
} = require("sillygirl");
`
}

const (
	NODE    = "node"
	PYTHON  = "python3"
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
	}
	files, err := os.ReadDir(home)
	if err == nil {
		indexes := []string{}
		for _, file := range files {
			if file.IsDir() || !strings.EqualFold(filepath.Ext(file.Name()), ".js") {
				continue
			}
			if file.Name() == "demo.main.js" {
				continue
			}
			indexes = append(indexes, filepath.Join(home, file.Name()))
		}
		if len(indexes) == 1 {
			return strings.ReplaceAll(indexes[0], "\\", "/"), NODE
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
	return "", false
}
