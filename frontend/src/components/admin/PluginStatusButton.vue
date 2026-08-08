<script setup lang="ts">
import Button from "ant-design-vue/es/button";
import { Power, PowerOff } from "lucide-vue-next";
import type { PluginInfo } from "../../types";

const props = defineProps<{
  record: PluginInfo;
  enabled: boolean;
  loading: boolean;
  blocked: boolean;
}>();
const emit = defineEmits<{ toggle: [] }>();
</script>

<template>
  <Button
    class="plugin-card-toggle"
    :class="enabled ? 'plugin-card-close' : 'plugin-card-enable'"
    shape="circle"
    :loading="loading"
    :disabled="blocked || loading"
    :title="`${enabled ? '关闭' : '开启'} ${props.record.title || props.record.id}`"
    :aria-label="`${enabled ? '关闭' : '开启'} ${props.record.title || props.record.id}`"
    @click="emit('toggle')"
  >
    <template #icon>
      <PowerOff v-if="enabled" :size="18" />
      <Power v-else :size="18" />
    </template>
  </Button>
</template>
