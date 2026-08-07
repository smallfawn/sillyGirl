<script setup lang="ts">
import Alert from "ant-design-vue/es/alert";
import {
  Antenna,
  Bot,
  CircleCheck,
  CircleX,
  MessageSquare,
  Pause,
  Play,
  QrCode,
  RefreshCw,
  Settings,
} from "lucide-vue-next";
import Button from "ant-design-vue/es/button";
import Empty from "ant-design-vue/es/empty";
import Form from "ant-design-vue/es/form";
import Input from "ant-design-vue/es/input";
import Modal from "ant-design-vue/es/modal";
import Segmented from "ant-design-vue/es/segmented";
import Space from "ant-design-vue/es/space";
import Spin from "ant-design-vue/es/spin";
import Switch from "ant-design-vue/es/switch";
import Tag from "ant-design-vue/es/tag";
import Typography from "ant-design-vue/es/typography";
import message from "ant-design-vue/es/message";
import { useAdminViewContext } from "../adminViewContext";

const {
  botEnabled,
  botSettings,
  botSettingsModal,
  botStatusRows,
  cancelCurrentBotSettings,
  clawbotLogin,
  closeClawbotLogin,
  login,
  oneBotReceiveURL,
  openBotSettings,
  page,
  pagermaidBridgeURL,
  qqGuildWebhookURL,
  refreshBots,
  saveCurrentBotSettings,
  setBotEnabled,
  settings,
  startClawbotLogin,
  submitClawbotVerifyCode,
  toggleWebChat,
  webChat,
  webChatEndpointURL,
} = useAdminViewContext();
</script>

<template>
  <section v-if="page === 'bots'" class="panel">
    <div class="toolbar">
      <div class="toolbar-left">
        <Bot :size="16" />
        <Typography.Text strong>BOT 对接管理</Typography.Text>
      </div>
      <div class="toolbar-right">
        <Button @click="refreshBots"
          ><template #icon><RefreshCw :size="16" /></template>刷新状态</Button
        >
      </div>
    </div>
    <Spin :spinning="botSettings.loading">
      <div class="bot-card-grid">
        <article
          v-for="record in botStatusRows"
          :key="record.platform"
          class="bot-card"
          :class="{
            'is-online': record.online,
            'is-disabled': !botEnabled(record),
          }"
        >
          <header class="bot-card-header">
            <span class="bot-card-avatar"><Bot :size="22" /></span>
            <div class="bot-card-heading">
              <strong>{{ record.label }}</strong>
              <span>{{ record.platform }}</span>
            </div>
            <div class="bot-card-actions">
              <template v-if="record.manageable !== false">
                <Button
                  v-if="botEnabled(record)"
                  class="bot-card-toggle bot-card-pause"
                  type="text"
                  shape="circle"
                  :title="`暂停 ${record.label}`"
                  :aria-label="`暂停 ${record.label}`"
                  @click="setBotEnabled(record, false)"
                >
                  <template #icon><Pause :size="18" /></template>
                </Button>
                <Button
                  v-else
                  class="bot-card-toggle bot-card-play"
                  type="text"
                  shape="circle"
                  :title="`开启 ${record.label}`"
                  :aria-label="`开启 ${record.label}`"
                  @click="setBotEnabled(record, true)"
                >
                  <template #icon><Play :size="18" /></template>
                </Button>
              </template>
              <Button
                class="bot-card-settings"
                type="text"
                shape="circle"
                :title="`${record.label}设置`"
                :aria-label="`${record.label}设置`"
                @click="openBotSettings(record)"
              >
                <template #icon><Settings :size="18" /></template>
              </Button>
            </div>
          </header>

          <div class="bot-card-status-list">
            <div class="bot-card-status">
              <CircleCheck
                v-if="botEnabled(record)"
                class="bot-status-enabled"
                :size="20"
                aria-hidden="true"
              />
              <CircleX
                v-else
                class="bot-status-disabled"
                :size="20"
                aria-hidden="true"
              />
              <span>启用状态</span>
              <strong
                :class="
                  botEnabled(record)
                    ? 'bot-status-enabled'
                    : 'bot-status-disabled'
                "
              >
                {{ botEnabled(record) ? "已启用" : "未启用" }}
              </strong>
            </div>
            <div class="bot-card-status">
              <Antenna
                :class="
                  record.online ? 'bot-status-online' : 'bot-status-offline'
                "
                :size="20"
                aria-hidden="true"
              />
              <span>连接状态</span>
              <strong
                :class="
                  record.online ? 'bot-status-online' : 'bot-status-offline'
                "
              >
                {{ record.online ? "已连接" : "未连接" }}
              </strong>
            </div>
          </div>

          <div class="bot-card-meta">
            <div>
              <span>实例数</span><strong>{{ record.count || 0 }}</strong>
            </div>
            <div>
              <span>Bot ID</span>
              <Typography.Text class="mono bot-card-bot-ids">{{
                record.bots_id?.length ? record.bots_id.join(", ") : "-"
              }}</Typography.Text>
            </div>
            <div v-if="record.manageable === false">
              <span>类型</span><Tag color="blue">内置 BOT</Tag>
            </div>
          </div>
        </article>
      </div>
    </Spin>
  </section>

  <Modal
    v-model:open="botSettingsModal.open"
    :title="`${botSettingsModal.label} 设置`"
    width="680px"
    ok-text="保存"
    cancel-text="取消"
    :confirm-loading="botSettings.saving"
    @cancel="cancelCurrentBotSettings"
    @ok="saveCurrentBotSettings"
  >
    <Spin :spinning="botSettings.loading">
      <Form
        v-if="botSettingsModal.platform === 'clawbot'"
        layout="vertical"
        class="bot-settings-modal-form"
      >
        <Form.Item label="启用 ClawBot" html-for="bot-clawbot-enable">
          <Switch
            id="bot-clawbot-enable"
            v-model:checked="botSettings.form.clawbot_enable"
          />
        </Form.Item>
        <Form.Item
          label="Token"
          html-for="bot-clawbot-token"
          extra="ClawBot / OpenClaw 微信通道的 iLink bot token。保存后适配器会自动重启。"
        >
          <Space.Compact style="width: 100%">
            <Input.Password
              id="bot-clawbot-token"
              v-model:value="botSettings.form.clawbot_token"
              name="clawbot-token"
              placeholder="请输入 ClawBot Token"
            />
            <Button :loading="clawbotLogin.starting" @click="startClawbotLogin">
              <template #icon><QrCode :size="16" /></template>扫码获取
            </Button>
          </Space.Compact>
        </Form.Item>
        <Form.Item
          label="API 地址"
          html-for="bot-clawbot-api"
          extra="默认使用腾讯 iLink API；如果你有兼容反代地址可以填写在这里。"
        >
          <Input
            id="bot-clawbot-api"
            v-model:value="botSettings.form.clawbot_api_base"
            name="clawbot-api"
            placeholder="https://ilinkai.weixin.qq.com"
          />
        </Form.Item>
        <Form.Item label="ClawBot 调试日志" html-for="bot-clawbot-debug">
          <Switch
            id="bot-clawbot-debug"
            v-model:checked="botSettings.form.clawbot_debug"
          />
        </Form.Item>
      </Form>

      <Form
        v-else-if="botSettingsModal.platform === 'qq'"
        layout="vertical"
        class="bot-settings-modal-form"
      >
        <Form.Item label="启用 QQ" html-for="bot-qq-enable">
          <Switch
            id="bot-qq-enable"
            v-model:checked="botSettings.form.qq_enable"
          />
        </Form.Item>
        <Form.Item label="反向 WebSocket 地址" html-for="bot-qq-receive">
          <Input id="bot-qq-receive" :value="oneBotReceiveURL" readonly>
            <template #suffix
              ><Typography.Text class="muted"
                >NapCat 填这个 URL</Typography.Text
              ></template
            >
          </Input>
        </Form.Item>
        <Form.Item
          label="连接密钥"
          html-for="bot-qq-token"
          extra="需要和 NapCat / OneBot 客户端配置里的 accessToken 保持一致；公网部署建议必须填写。"
        >
          <Input.Password
            id="bot-qq-token"
            v-model:value="botSettings.form.qq_token"
            name="qq-token"
            placeholder="请输入 QQ 连接密钥"
          />
        </Form.Item>
        <Form.Item label="QQ 调试日志" html-for="bot-qq-debug">
          <Switch
            id="bot-qq-debug"
            v-model:checked="botSettings.form.qq_debug"
          />
        </Form.Item>
      </Form>

      <Form
        v-else-if="botSettingsModal.platform === 'telegram'"
        layout="vertical"
        class="bot-settings-modal-form"
      >
        <Form.Item label="启用 Telegram" html-for="bot-telegram-enable">
          <Switch
            id="bot-telegram-enable"
            v-model:checked="botSettings.form.telegram_enable"
          />
        </Form.Item>
        <Form.Item
          label="Token"
          html-for="bot-telegram-token"
          extra="BotFather 提供的 Bot Token，保存后 Telegram 适配器会自动重启。"
        >
          <Input.Password
            id="bot-telegram-token"
            v-model:value="botSettings.form.telegram_token"
            name="telegram-token"
            placeholder="123456:ABC-DEF..."
          />
        </Form.Item>
        <Form.Item
          label="代理 API"
          html-for="bot-telegram-api"
          extra="默认使用 https://api.telegram.org；网络不通时填写兼容反代地址。"
        >
          <Input
            id="bot-telegram-api"
            v-model:value="botSettings.form.telegram_api_base"
            name="telegram-api"
            placeholder="https://api.telegram.org"
          />
        </Form.Item>
        <Form.Item label="Telegram 调试日志" html-for="bot-telegram-debug">
          <Switch
            id="bot-telegram-debug"
            v-model:checked="botSettings.form.telegram_debug"
          />
        </Form.Item>
      </Form>

      <Form
        v-else-if="botSettingsModal.platform === 'dingtalk'"
        layout="vertical"
        class="bot-settings-modal-form"
      >
        <Form.Item label="启用钉钉" html-for="bot-dingtalk-enable">
          <Switch
            id="bot-dingtalk-enable"
            v-model:checked="botSettings.form.dingtalk_enable"
          />
        </Form.Item>
        <Form.Item
          label="Client ID"
          html-for="bot-dingtalk-client-id"
          extra="钉钉开放平台应用的 Client ID（原 AppKey）。适配器使用 Stream 模式，不需要公网回调地址。"
        >
          <Input
            id="bot-dingtalk-client-id"
            v-model:value="botSettings.form.dingtalk_client_id"
            name="dingtalk-client-id"
            placeholder="dingxxxxxxxx"
          />
        </Form.Item>
        <Form.Item label="Client Secret" html-for="bot-dingtalk-secret">
          <Input.Password
            id="bot-dingtalk-secret"
            v-model:value="botSettings.form.dingtalk_client_secret"
            name="dingtalk-client-secret"
            placeholder="请输入 Client Secret"
          />
        </Form.Item>
        <Form.Item label="钉钉调试日志" html-for="bot-dingtalk-debug">
          <Switch
            id="bot-dingtalk-debug"
            v-model:checked="botSettings.form.dingtalk_debug"
          />
        </Form.Item>
      </Form>

      <Form
        v-else-if="botSettingsModal.platform === 'qqguild'"
        layout="vertical"
        class="bot-settings-modal-form"
      >
        <Form.Item label="启用 QQ 官方频道" html-for="bot-qqguild-enable">
          <Switch
            id="bot-qqguild-enable"
            v-model:checked="botSettings.form.qqguild_enable"
          />
        </Form.Item>
        <fieldset class="bot-mode-fieldset">
          <legend id="bot-qqguild-mode-label">接入模式</legend>
          <Segmented
            v-model:value="botSettings.form.qqguild_mode"
            block
            role="radiogroup"
            aria-labelledby="bot-qqguild-mode-label"
            :options="[
              { label: 'Webhook', value: 'webhook' },
              { label: 'WebSocket', value: 'websocket' },
            ]"
          />
        </fieldset>
        <Form.Item
          v-if="botSettings.form.qqguild_mode === 'webhook'"
          label="Webhook 回调地址"
          html-for="bot-qqguild-webhook"
          extra="把该地址填入 QQ 开放平台机器人事件回调；HTTPS 由反向代理提供。"
        >
          <Input id="bot-qqguild-webhook" :value="qqGuildWebhookURL" readonly />
        </Form.Item>
        <Alert
          v-else
          type="info"
          show-icon
          message="WebSocket 主动连接 QQ Gateway，不需要公网回调地址。"
          style="margin-bottom: 18px"
        />
        <Form.Item label="AppID" html-for="bot-qqguild-app-id">
          <Input
            id="bot-qqguild-app-id"
            v-model:value="botSettings.form.qqguild_app_id"
            name="qqguild-app-id"
            placeholder="请输入机器人 AppID"
          />
        </Form.Item>
        <Form.Item label="AppSecret" html-for="bot-qqguild-secret">
          <Input.Password
            id="bot-qqguild-secret"
            v-model:value="botSettings.form.qqguild_app_secret"
            name="qqguild-app-secret"
            placeholder="请输入机器人 AppSecret"
          />
        </Form.Item>
        <Form.Item label="沙箱环境" html-for="bot-qqguild-sandbox">
          <Switch
            id="bot-qqguild-sandbox"
            v-model:checked="botSettings.form.qqguild_sandbox"
          />
        </Form.Item>
        <Form.Item label="QQ 频道调试日志" html-for="bot-qqguild-debug">
          <Switch
            id="bot-qqguild-debug"
            v-model:checked="botSettings.form.qqguild_debug"
          />
        </Form.Item>
      </Form>

      <Form
        v-else-if="botSettingsModal.platform === 'web'"
        layout="vertical"
        class="bot-settings-modal-form"
      >
        <Form.Item
          label="运行状态"
          html-for="bot-web-status"
          extra="Web Bot 是内置适配器，随 SillyGirl 自动启动。"
        >
          <Switch id="bot-web-status" :checked="true" disabled />
        </Form.Item>
        <Form.Item
          label="允许匿名聊天"
          html-for="bot-web-public"
          extra="关闭时仅已登录的后台管理员可以发送 Web Bot 消息。"
        >
          <Switch
            id="bot-web-public"
            v-model:checked="botSettings.form.web_chat_public"
          />
        </Form.Item>
        <Form.Item label="聊天接口" html-for="bot-web-endpoint">
          <Input id="bot-web-endpoint" :value="webChatEndpointURL" readonly />
        </Form.Item>
        <Button type="primary" @click="toggleWebChat">
          <template #icon><MessageSquare :size="16" /></template>
          {{ webChat.open ? "关闭聊天窗口" : "打开聊天窗口" }}
        </Button>
      </Form>

      <Form v-else layout="vertical" class="bot-settings-modal-form">
        <Form.Item label="启用 Pagermaid" html-for="bot-pagermaid-enable">
          <Switch
            id="bot-pagermaid-enable"
            v-model:checked="botSettings.form.pagermaid_enable"
          />
        </Form.Item>
        <Form.Item label="连接密钥" html-for="bot-pagermaid-token">
          <Input.Password
            id="bot-pagermaid-token"
            v-model:value="botSettings.form.pagermaid_token"
            name="pagermaid-token"
            placeholder="可选，建议填写"
          />
        </Form.Item>
        <Form.Item label="Pagermaid 调试日志" html-for="bot-pagermaid-debug">
          <Switch
            id="bot-pagermaid-debug"
            v-model:checked="botSettings.form.pagermaid_debug"
          />
        </Form.Item>
        <div class="bot-settings-readonly">
          <Typography.Text class="muted block">桥接脚本</Typography.Text>
          <Typography.Text class="mono block"
            >adapters/pagermaid/sillyplus.py</Typography.Text
          >
        </div>
        <div class="bot-settings-readonly">
          <Typography.Text class="muted block">WebSocket 地址</Typography.Text>
          <Typography.Text class="mono block">{{
            pagermaidBridgeURL
          }}</Typography.Text>
        </div>
      </Form>
    </Spin>
  </Modal>

  <Modal
    v-model:open="clawbotLogin.open"
    title="扫码获取 ClawBot Token"
    :footer="null"
    @cancel="closeClawbotLogin"
  >
    <div class="clawbot-login-modal">
      <div class="clawbot-qr-frame">
        <Spin :spinning="clawbotLogin.starting">
          <img
            v-if="clawbotLogin.qrcodeImg"
            :src="clawbotLogin.qrcodeImg"
            alt="ClawBot 登录二维码"
          />
          <Empty
            v-else
            :description="clawbotLogin.message || '等待生成二维码'"
          />
        </Spin>
      </div>
      <Typography.Text>{{
        clawbotLogin.message || "请使用微信扫码"
      }}</Typography.Text>
      <div v-if="clawbotLogin.needVerify" class="clawbot-verify-row">
        <Input
          v-model:value="clawbotLogin.verifyCode"
          placeholder="输入手机微信显示的数字"
          @pressEnter="submitClawbotVerifyCode"
        />
        <Button
          type="primary"
          :loading="clawbotLogin.polling"
          @click="submitClawbotVerifyCode"
          >确认</Button
        >
      </div>
      <Space>
        <Button :loading="clawbotLogin.starting" @click="startClawbotLogin">
          <template #icon><RefreshCw :size="16" /></template>
          重新生成
        </Button>
        <Button @click="closeClawbotLogin">关闭</Button>
      </Space>
    </div>
  </Modal>
</template>
