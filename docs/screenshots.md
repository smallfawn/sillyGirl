# 界面截图

[返回项目导航](../README.md) · [API 与存储](api-storage.md) · [插件编写](plugin-development.md) · [适配器](adapters.md)

以下截图由 **Chrome DevTools MCP** 在 SillyGirl `v1.0.9` 的隔离本地 fixture 上生成，页面数据仅用于文档展示。

## 页面总览

| 页面 | 路由 | 用途 |
|---|---|---|
| 管理概览 | `/admin` | 查看版本、脚本、用户与容器摘要 |
| 插件市场 | `/admin/plugins` | 搜索、安装、编辑和配置插件 |
| 存储管理 | `/admin/storage` | 查看 Bucket、Key 与 Value |
| BOT 管理 | `/admin/bots` | 配置适配器并查看连接状态 |

## 管理概览

![SillyGirl 管理概览](images/admin-overview.png)

概览页集中显示当前版本、远端版本、脚本数量、用户数量以及三类容器面板数量。

## 插件市场

![SillyGirl 插件市场](images/plugin-market.png)

插件卡片提供源码编辑、详情、安装、卸载和配置入口；筛选、搜索与编辑器按需加载。

## 存储管理

![SillyGirl 存储管理](images/storage-management.png)

存储页按 Bucket 浏览键值，支持查询、新建、修改和删除。截图使用 `demo` Bucket，不含生产配置。

## 适配器管理

![SillyGirl 适配器管理](images/adapter-management.png)

BOT 页面统一展示微信 ClawBot、钉钉、Pagermaid、QQ、QQ 官方频道、Web Bot 与 Telegram 的启用和连接状态。
