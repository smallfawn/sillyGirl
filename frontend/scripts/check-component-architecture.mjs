import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, resolve } from "node:path";

const root = resolve(import.meta.dirname, "..");
const failures = [];

function read(path) {
  return readFileSync(join(root, path), "utf8");
}

function lines(path) {
  return read(path).split(/\r?\n/).length;
}

function check(condition, message) {
  if (!condition) failures.push(message);
}

function sourceFiles(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name);
    return entry.isDirectory() ? sourceFiles(path) : [path];
  });
}

const views = [
  "WelcomeView",
  "BotsView",
  "DependenciesView",
  "StorageView",
  "UsersView",
  "MessageToolsView",
  "MastersView",
  "TasksView",
  "ContainersView",
  "PluginsView",
  "SettingsView",
];

const app = read("src/App.vue");
check(
  lines("src/App.vue") <= 180,
  `App.vue 超过 180 行：${lines("src/App.vue")}`,
);
check(
  app.includes('<component :is="activeView" />'),
  "App.vue 必须只渲染当前功能视图",
);
for (const view of views) {
  const path = `src/components/admin/views/${view}.vue`;
  check(statSync(join(root, path)).isFile(), `缺少功能视图：${path}`);
  const lazyImport = new RegExp(
    `defineAsyncComponent\\([\\s\\S]{0,100}?import\\(["']\\./components/admin/views/${view}\\.vue["']\\)`,
  );
  check(lazyImport.test(app), `${view} 未按路由懒加载`);
  check(
    !new RegExp(`import\\s+${view}\\s+from`).test(app),
    `${view} 被静态导入，破坏按功能拆包`,
  );
  check(lines(path) <= 700, `${path} 超过 700 行：${lines(path)}`);
}

const composableDir = join(root, "src/composables/admin");
const composables = readdirSync(composableDir).filter((name) =>
  /^use.+Admin\.ts$/.test(name),
);
check(
  composables.length >= 8,
  `admin domain composable 少于 8 个：${composables.length}`,
);
check(
  lines("src/composables/admin/useAdminController.ts") <= 3400,
  `useAdminController.ts 超过 3400 行：${lines("src/composables/admin/useAdminController.ts")}`,
);

const context = read("src/components/admin/adminViewContext.ts");
check(
  context.includes("ReturnType<typeof useAdminController>"),
  "admin view context 必须由 controller 返回类型推导",
);
const controller = read("src/composables/admin/useAdminController.ts");
check(
  !/^import\s+(?!type\s).*from\s+["'](?:@codemirror|codemirror)/m.test(
    controller,
  ),
  "CodeMirror 必须在首次打开插件编辑器时动态加载",
);
const panelsAdmin = read("src/composables/admin/usePanelsAdmin.ts");
const containersView = read("src/components/admin/views/ContainersView.vue");
check(
  panelsAdmin.includes("let panelsLoaded = false") &&
    panelsAdmin.includes("let panelsRequest"),
  "统一面板接口必须缓存成功结果并复用进行中的请求",
);
check(
  panelsAdmin.includes("refreshActiveContainerPanels"),
  "容器列表的显式刷新必须与页面/Tab 初次加载分离",
);
const containerWatcher = controller.match(
  /watch\(containerKind,[\s\S]*?\n\s*\}\);/,
)?.[0];
check(
  !!containerWatcher && !containerWatcher.includes("loadActiveContainerPanels"),
  "切换容器 Tab 不得重复请求统一面板接口",
);
check(
  containersView.includes('@click="refreshActiveContainerPanels"'),
  "刷新按钮必须使用显式刷新函数",
);
for (const page of ["src/App.vue", "src/Home.vue", "src/User.vue"]) {
  check(
    read(page).includes("components/common/AppBrand.vue"),
    `${page} 未复用 AppBrand`,
  );
}

for (const path of sourceFiles(join(root, "src"))) {
  if (!/\.(?:ts|vue|js)$/.test(path)) continue;
  const source = readFileSync(path, "utf8");
  check(
    !/method\s*:\s*["'](?:PUT|PATCH|DELETE)["']/.test(source),
    `${path.slice(root.length + 1)} 使用了 GET/POST 之外的 API 方法`,
  );
}

if (failures.length) {
  console.error(`组件架构检查失败（${failures.length}）：`);
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(
  JSON.stringify(
    {
      app_lines: lines("src/App.vue"),
      lazy_views: views.length,
      domain_composables: composables.length,
      controller_lines: lines("src/composables/admin/useAdminController.ts"),
    },
    null,
    2,
  ),
);
