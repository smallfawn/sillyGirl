<script setup lang="ts">
import { computed, defineAsyncComponent, provide } from "vue";
import AntApp from "ant-design-vue/es/app";
import Button from "ant-design-vue/es/button";
import ConfigProvider from "ant-design-vue/es/config-provider";
import Drawer from "ant-design-vue/es/drawer";
import Layout from "ant-design-vue/es/layout";
import Menu from "ant-design-vue/es/menu";
import Typography from "ant-design-vue/es/typography";
import zhCN from "ant-design-vue/es/locale/zh_CN";
import { LogOut, Menu as MenuIcon } from "lucide-vue-next";
import AdminAuth from "./components/admin/AdminAuth.vue";
import AdminWebChat from "./components/admin/AdminWebChat.vue";
import { adminViewContextKey } from "./components/admin/adminViewContext";
import AppBrand from "./components/common/AppBrand.vue";
import { useAdminController } from "./composables/admin/useAdminController";

const controller = useAdminController();
provide(adminViewContextKey, controller);

const { booting, logout, menuItems, mobileMenuOpen, navigate, page, user } =
  controller;

const adminViews = {
  welcome: defineAsyncComponent(
    () => import("./components/admin/views/WelcomeView.vue"),
  ),
  bots: defineAsyncComponent(
    () => import("./components/admin/views/BotsView.vue"),
  ),
  dependencies: defineAsyncComponent(
    () => import("./components/admin/views/DependenciesView.vue"),
  ),
  storage: defineAsyncComponent(
    () => import("./components/admin/views/StorageView.vue"),
  ),
  users: defineAsyncComponent(
    () => import("./components/admin/views/UsersView.vue"),
  ),
  "message-tools": defineAsyncComponent(
    () => import("./components/admin/views/MessageToolsView.vue"),
  ),
  masters: defineAsyncComponent(
    () => import("./components/admin/views/MastersView.vue"),
  ),
  tasks: defineAsyncComponent(
    () => import("./components/admin/views/TasksView.vue"),
  ),
  containers: defineAsyncComponent(
    () => import("./components/admin/views/ContainersView.vue"),
  ),
  plugins: defineAsyncComponent(
    () => import("./components/admin/views/PluginsView.vue"),
  ),
  settings: defineAsyncComponent(
    () => import("./components/admin/views/SettingsView.vue"),
  ),
};

const activeView = computed(() => adminViews[page.value] || adminViews.welcome);
</script>

<template>
  <ConfigProvider :locale="zhCN">
    <AntApp>
      <AdminAuth v-if="!booting && !user" />

      <Layout v-else class="shell">
        <Layout.Sider class="desktop-sider" :width="220" theme="light">
          <AppBrand class="brand" />
          <Menu
            mode="inline"
            :selected-keys="[page]"
            :items="menuItems"
            style="border-inline-end: 0; padding-top: 8px"
            @click="(e: any) => navigate(e.key)"
          />
        </Layout.Sider>
        <Layout>
          <div class="topbar">
            <div class="topbar-title">
              <Button
                class="mobile-menu-button"
                type="text"
                @click="mobileMenuOpen = true"
              >
                <template #icon><MenuIcon :size="18" /></template>
              </Button>
              <div class="topbar-heading">
                <Typography.Text strong>{{
                  menuItems.find((item) => item.key === page)?.label || "后台"
                }}</Typography.Text>
                <Typography.Text class="muted" style="margin-left: 10px">{{
                  user?.name || "傻妞"
                }}</Typography.Text>
              </div>
            </div>
            <Button class="logout-button" @click="logout"
              ><template #icon><LogOut :size="16" /></template>退出</Button
            >
          </div>
          <main class="content">
            <component :is="activeView" />
          </main>
        </Layout>

        <Drawer
          v-model:open="mobileMenuOpen"
          class="mobile-menu-drawer"
          placement="left"
          :width="280"
          :body-style="{ padding: 0 }"
          :closable="false"
        >
          <AppBrand class="brand" />
          <Menu
            mode="inline"
            :selected-keys="[page]"
            :items="menuItems"
            style="border-inline-end: 0; padding-top: 8px"
            @click="(e: any) => navigate(e.key)"
          />
        </Drawer>
      </Layout>

      <AdminWebChat />
    </AntApp>
  </ConfigProvider>
</template>
