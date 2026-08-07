# 前端组件架构

## 页面入口

- `src/App.vue`：后台 shell，只负责认证态分支、导航、当前功能视图和依赖注入。
- `src/Home.vue`：公开首页。
- `src/User.vue`：普通用户中心。
- 三个入口共用 `src/components/common/AppBrand.vue`，避免重复维护品牌结构和 Logo 样式。

## 后台功能视图

后台业务视图位于 `src/components/admin/views/`，按菜单资源拆分：概览、BOT、依赖、存储、用户、消息工具、管理员、任务、容器、插件和设置。

`App.vue` 使用 `defineAsyncComponent` 按当前菜单加载视图。新增后台菜单时必须：

1. 在 `src/components/admin/views/` 新建独立 SFC；
2. 在 `App.vue` 的 `adminViews` 中注册动态 import；
3. 在 controller 的 `PageKey`、`validPages` 和菜单配置中注册资源；
4. 运行 `npm run check:components` 和 `npm run build`。

## 状态与业务逻辑

- `src/composables/admin/useAdminController.ts` 负责编排认证、路由、插件市场、BOT 和跨功能刷新。
- 独立资源逻辑位于 `src/composables/admin/use*Admin.ts`，目前已拆分 storage、users、tasks、panels、replies、masters、carry 和 message rules。
- `src/components/admin/adminViewContext.ts` 使用 `ReturnType<typeof useAdminController>` 推导上下文类型；功能视图通过 `useAdminViewContext()` 获取同一控制器实例，不复制状态。
- `src/composables/admin/adminApi.ts` 统一处理 API envelope 解包。
- 前端业务请求只发送 `GET` 和 `POST`；更新向资源 URI 提交 `POST`，删除向对应的 `deletions` 子资源提交 `POST`。

## 架构约束

`scripts/check-component-architecture.mjs` 在构建前检查：

- 根 `App.vue` 不重新膨胀；
- 11 个后台视图保持动态加载，禁止退回静态全量 import；
- 单个功能视图和 controller 不超过当前上限；
- domain composables、typed context 和公共品牌组件继续存在。
- `src/` 内不得重新引入 `PUT`、`PATCH` 或 `DELETE` 请求方法。

这些约束已接入 `npm run build`，架构退化会直接中止构建。
