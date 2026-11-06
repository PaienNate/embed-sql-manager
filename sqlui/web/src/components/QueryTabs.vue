<template>
  <div class="query-tabs">
    <div class="query-tabs-scroll">
      <div
        v-for="tab in tabs"
        :key="tab.id"
        class="query-tab"
        :class="{ active: tab.id === activeId }"
        @click="$emit('select', tab.id)">
        <i class="fa fa-file-text-o"></i>
        <span class="query-tab-title" :title="tab.title">{{ tab.title }}</span>
        <i
          v-if="tabs.length > 1"
          class="fa fa-times query-tab-close"
          title="关闭"
          @click.stop="$emit('close', tab.id)">
        </i>
      </div>
      <div class="query-tab-add" title="新建查询" @click="$emit('add')">
        <i class="fa fa-plus"></i>
      </div>
    </div>
    <div class="query-tab-actions">
      <button type="button" class="query-action" title="导入 SQL 文件" @click="$emit('import')">
        <i class="fa fa-upload"></i> 导入
      </button>
      <button type="button" class="query-action" title="导出当前查询" @click="$emit('export')">
        <i class="fa fa-download"></i> 导出
      </button>
      <button type="button" class="query-action" title="导出全部查询" @click="$emit('export-all')">
        <i class="fa fa-download"></i> 全部
      </button>
    </div>
  </div>
</template>

<script>
export default {
  props: {
    tabs: { type: Array, default: () => [] },
    activeId: { type: String, default: '' },
  },
};
</script>
