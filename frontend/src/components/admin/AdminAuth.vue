<script setup lang="ts">
import Button from "ant-design-vue/es/button";
import Form from "ant-design-vue/es/form";
import Input from "ant-design-vue/es/input";
import Typography from "ant-design-vue/es/typography";
import { User } from "lucide-vue-next";
import { useAdminViewContext } from "./adminViewContext";

const {
  booting,
  login,
  loginModel,
  page,
  setupAdmin,
  setupModel,
  setupRequired,
  user,
} = useAdminViewContext();
</script>

<template>
  <div v-if="!booting && !user" class="login-page">
    <div class="login-card">
      <template v-if="setupRequired">
        <Typography.Title :level="3" style="margin-top: 0"
          >初始化管理员</Typography.Title
        >
        <Typography.Paragraph class="muted"
          >首次使用需要创建后台账号和密码。</Typography.Paragraph
        >
        <Form layout="vertical" @finish="setupAdmin">
          <Form.Item label="账号" required>
            <Input
              id="setup-username"
              name="setup-username"
              v-model:value="setupModel.username"
            >
              <template #prefix><User :size="16" /></template>
            </Input>
          </Form.Item>
          <Form.Item label="密码" required>
            <Input.Password
              id="setup-password"
              name="setup-password"
              v-model:value="setupModel.password"
            />
          </Form.Item>
          <Form.Item label="确认密码" required>
            <Input.Password
              id="setup-confirm"
              name="setup-confirm"
              v-model:value="setupModel.confirm"
            />
          </Form.Item>
          <Button type="primary" html-type="button" block @click="setupAdmin"
            >创建账号</Button
          >
        </Form>
      </template>
      <template v-else>
        <Typography.Title :level="3" style="margin-top: 0"
          >SillyGirl Admin</Typography.Title
        >
        <Typography.Paragraph class="muted"
          >使用后台账号和密码登录。</Typography.Paragraph
        >
        <Form layout="vertical" @finish="login">
          <Form.Item label="账号" required>
            <Input
              id="login-username"
              name="login-username"
              autocomplete="username"
              v-model:value="loginModel.username"
            >
              <template #prefix><User :size="16" /></template>
            </Input>
          </Form.Item>
          <Form.Item label="密码" required>
            <Input.Password
              id="login-password"
              name="login-password"
              autocomplete="current-password"
              v-model:value="loginModel.password"
            />
          </Form.Item>
          <Button type="primary" html-type="button" block @click="login"
            >登录</Button
          >
        </Form>
      </template>
    </div>
  </div>
</template>
