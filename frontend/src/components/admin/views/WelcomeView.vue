<script setup lang="ts">
import Alert from "ant-design-vue/es/alert";
import Button from "ant-design-vue/es/button";
import Card from "ant-design-vue/es/card";
import { CloudDownload } from "lucide-vue-next";
import Col from "ant-design-vue/es/col";
import Modal from "ant-design-vue/es/modal";
import Progress from "ant-design-vue/es/progress";
import Row from "ant-design-vue/es/row";
import Space from "ant-design-vue/es/space";
import Statistic from "ant-design-vue/es/statistic";
import Tag from "ant-design-vue/es/tag";
import Typography from "ant-design-vue/es/typography";
import message from "ant-design-vue/es/message";
import { useAdminViewContext } from "../adminViewContext";

const {
  daidai,
  overviewIntegrations,
  overviewUserStats,
  overviewVersion,
  page,
  qinglong,
  realScripts,
  restartAfterUpdate,
  smallcat,
  startOnlineUpdate,
  systemUpdate,
  user,
} = useAdminViewContext();
</script>

<template>
  <section v-if="page === 'welcome'" class="panel">
    <Typography.Title :level="3" style="margin-top: 0">{{
      user?.name || "傻妞"
    }}</Typography.Title>
    <Space wrap style="margin-bottom: 14px">
      <Tag color="blue">当前版本 {{ overviewVersion.local }}</Tag>
      <Tag color="green">最新版本 {{ overviewVersion.remote }}</Tag>
      <Typography.Link :href="overviewVersion.repository" target="_blank"
        >GitHub</Typography.Link
      >
      <Button
        type="primary"
        size="small"
        :loading="systemUpdate.running"
        @click="startOnlineUpdate"
      >
        <template #icon><CloudDownload :size="15" /></template>
        在线更新
      </Button>
    </Space>
    <Row :gutter="[12, 12]">
      <Col :xs="24" :sm="12" :md="8"
        ><Card><Statistic title="脚本数量" :value="realScripts.length" /></Card
      ></Col>
      <Col :xs="24" :sm="12" :md="8"
        ><Card
          ><Statistic
            title="今日新增用户"
            :value="overviewUserStats.today" /></Card
      ></Col>
      <Col :xs="24" :sm="12" :md="8"
        ><Card
          ><Statistic
            title="总用户数量"
            :value="overviewUserStats.total" /></Card
      ></Col>
      <Col :xs="24" :sm="12" :md="8"
        ><Card
          ><Statistic
            title="青龙容器"
            :value="
              overviewIntegrations.find((item) => item.key === 'qinglong')
                ?.count || 0
            " /></Card
      ></Col>
      <Col :xs="24" :sm="12" :md="8"
        ><Card
          ><Statistic
            title="smallcat"
            :value="
              overviewIntegrations.find((item) => item.key === 'smallcat')
                ?.count || 0
            " /></Card
      ></Col>
      <Col :xs="24" :sm="12" :md="8"
        ><Card
          ><Statistic
            title="呆呆容器"
            :value="
              overviewIntegrations.find((item) => item.key === 'daidai')
                ?.count || 0
            " /></Card
      ></Col>
    </Row>
  </section>

  <Modal
    v-model:open="systemUpdate.open"
    title="在线更新"
    :footer="null"
    :closable="!systemUpdate.running && !systemUpdate.restartChecking"
    :mask-closable="!systemUpdate.running && !systemUpdate.restartChecking"
  >
    <Space direction="vertical" style="width: 100%" size="middle">
      <Progress
        :percent="systemUpdate.percent"
        :status="
          systemUpdate.status === 'error'
            ? 'exception'
            : systemUpdate.status === 'done'
              ? 'success'
              : 'active'
        "
      />
      <Alert
        :type="
          systemUpdate.status === 'error'
            ? 'error'
            : systemUpdate.status === 'done'
              ? 'success'
              : 'info'
        "
        :message="systemUpdate.message || '准备更新'"
        show-icon
      />
      <div v-if="systemUpdate.result" class="update-result">
        <Typography.Text class="block"
          >版本：{{ systemUpdate.result.before || "-" }} ->
          {{ systemUpdate.result.after || "-" }}</Typography.Text
        >
        <Typography.Text class="block"
          >文件：{{ systemUpdate.result.asset || "-" }}</Typography.Text
        >
        <Typography.Text class="block muted">{{
          systemUpdate.result.output || ""
        }}</Typography.Text>
      </div>
      <Space
        v-if="systemUpdate.status === 'done' && !systemUpdate.restartChecking"
        style="justify-content: flex-end; width: 100%"
      >
        <Button @click="systemUpdate.open = false">关闭</Button>
        <Button
          v-if="systemUpdate.result"
          type="primary"
          :loading="systemUpdate.restarting"
          @click="restartAfterUpdate"
          >立即重启</Button
        >
      </Space>
      <Space
        v-if="systemUpdate.status === 'error'"
        style="justify-content: flex-end; width: 100%"
      >
        <Button @click="systemUpdate.open = false">关闭</Button>
      </Space>
    </Space>
  </Modal>
</template>
