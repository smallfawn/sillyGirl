<script setup lang="ts">
import { Bot, MessageSquare, Send, X } from "lucide-vue-next";
import Button from "ant-design-vue/es/button";
import Input from "ant-design-vue/es/input";
import message from "ant-design-vue/es/message";
import { ref } from "vue";
import { useAdminViewContext } from "./adminViewContext";

const { sendWebChat, toggleWebChat, user, webChat, webChatMessagesEl } =
  useAdminViewContext();
</script>

<template>
  <div v-if="user" class="web-chat-widget">
    <section
      v-if="webChat.open"
      class="web-chat-panel"
      role="dialog"
      aria-label="Web Bot 对话框"
    >
      <header class="web-chat-header">
        <div class="web-chat-title">
          <span class="web-chat-avatar"><Bot :size="19" /></span>
          <div>
            <strong>Web Bot</strong>
            <span class="web-chat-status">
              <i :class="{ online: webChat.polling }"></i>
              {{ webChat.polling ? "在线" : "连接中" }}
            </span>
          </div>
        </div>
        <button
          type="button"
          class="web-chat-close"
          aria-label="关闭 Web Bot"
          @click="toggleWebChat"
        >
          <X :size="18" />
        </button>
      </header>

      <div ref="webChatMessagesEl" class="web-chat-messages" aria-live="polite">
        <div
          v-for="item in webChat.messages"
          :key="item.id"
          class="web-chat-message-row"
          :class="{ own: item.own, notice: item.t === 'notice' }"
        >
          <div class="web-chat-bubble">
            <div v-if="item.c" class="web-chat-content">{{ item.c }}</div>
            <div v-if="item.m?.length" class="web-chat-images">
              <a
                v-for="imageURL in item.m"
                :key="imageURL"
                :href="imageURL"
                target="_blank"
                rel="noreferrer"
              >
                <img :src="imageURL" alt="Web Bot 返回图片" />
              </a>
            </div>
          </div>
        </div>
      </div>

      <div v-if="webChat.error" class="web-chat-error">{{ webChat.error }}</div>
      <footer class="web-chat-composer">
        <Input.TextArea
          id="web-chat-input"
          name="web-chat-input"
          v-model:value="webChat.input"
          :auto-size="{ minRows: 1, maxRows: 4 }"
          :maxlength="2000"
          aria-label="Web Bot 消息"
          placeholder="输入命令，Enter 发送"
          @keydown.enter.exact.prevent="sendWebChat"
        />
        <Button
          type="primary"
          shape="circle"
          aria-label="发送消息"
          :loading="webChat.sending"
          :disabled="!webChat.input.trim()"
          @click="sendWebChat"
        >
          <template #icon><Send :size="17" /></template>
        </Button>
      </footer>
    </section>

    <button
      type="button"
      class="web-chat-fab"
      :class="{ active: webChat.open }"
      :aria-expanded="webChat.open"
      :aria-label="webChat.open ? '关闭 Web Bot' : '打开 Web Bot'"
      @click="toggleWebChat"
    >
      <X v-if="webChat.open" :size="24" />
      <MessageSquare v-else :size="25" />
      <span v-if="webChat.unread" class="web-chat-unread">{{
        webChat.unread > 99 ? "99+" : webChat.unread
      }}</span>
    </button>
  </div>
</template>
