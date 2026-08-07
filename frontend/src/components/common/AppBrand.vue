<script setup lang="ts">
withDefaults(
  defineProps<{
    href?: string;
    label?: string;
  }>(),
  {
    href: "",
    label: "SillyGirl",
  },
);
</script>

<template>
  <a v-if="href" :href="href" class="app-brand">
    <span class="app-brand-mark" role="img" aria-label="傻妞 Logo"></span>
    <span>{{ label }}</span>
  </a>
  <div v-else class="app-brand">
    <span class="app-brand-mark" role="img" aria-label="傻妞 Logo"></span>
    <span>{{ label }}</span>
  </div>
</template>

<style scoped>
.app-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  color: #1f2937;
  font-weight: 700;
  text-decoration: none;
}

.app-brand-mark {
  display: grid;
  flex: 0 0 auto;
  place-items: center;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  background: #fff url("../../../logo.png") center 45% / 150% auto no-repeat;
  box-shadow: 0 1px 4px rgb(148 81 48 / 18%);
}
</style>
