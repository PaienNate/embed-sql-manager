<template>
  <div v-loading="loading" class="result-panel">
    <el-tabs v-model="activeTab" class="result-tabs">
      <el-tab-pane label="数据" name="data">
        <div class="result-pane">
          <div v-if="table || columns.length" class="result-toolbar">
            <span class="result-table-name">
              <i class="fa fa-table"></i>{{ table ? table.table : '查询结果' }}
            </span>
            <span v-if="result && result.truncated" class="result-warn">仅显示前 {{ rows.length }} 行</span>
            <span v-if="columns.length" class="result-count">{{ rows.length }} 行</span>
            <span class="spacer"></span>
            <el-button v-if="table" size="mini" @click="$emit('refresh')">
              <i class="fa fa-refresh"></i> 刷新
            </el-button>
            <el-button v-if="columns.length" size="mini" @click="exportCSV">
              <i class="fa fa-download"></i> CSV
            </el-button>
            <el-button v-if="columns.length" size="mini" @click="exportJSON">
              <i class="fa fa-download"></i> JSON
            </el-button>
          </div>

          <div v-if="isExecResult" class="exec-message">
            <i class="fa fa-check"></i>
            执行成功，影响 {{ result.rowsAffected }} 行
            <span v-if="result.lastInsertId">，自增 ID: {{ result.lastInsertId }}</span>
          </div>

          <div v-else-if="columns.length" class="grid-wrap">
            <vxe-table
              ref="grid"
              :data="rows"
              size="mini"
              border
              height="100%"
              :row-config="{ isHover: true, height: 28 }"
              :column-config="{ resizable: true }"
              :scroll-y="{ enabled: true }"
              show-overflow>
              <vxe-column type="seq" width="50"></vxe-column>
              <vxe-column
                v-for="(col, i) in columns"
                :key="i"
                :field="String(i)"
                :title="col.name"
                min-width="120">
                <template #default="{ row }">
                  <span v-if="row[i] === null" class="cell-null">NULL</span>
                  <span v-else-if="col.binary" class="cell-binary">&lt;binary&gt;</span>
                  <span v-else>{{ row[i] }}</span>
                </template>
              </vxe-column>
            </vxe-table>
          </div>

          <div v-else class="empty-state">
            <i class="fa fa-table"></i>
            <p>选择左侧的表，或在上方执行 SQL</p>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane v-if="table || structure.length" label="结构" name="structure">
        <div class="structure-wrap">
          <el-table :data="structure" size="mini" border height="100%">
            <el-table-column prop="name" label="字段" min-width="150"></el-table-column>
            <el-table-column prop="type" label="类型" min-width="130"></el-table-column>
            <el-table-column label="可空" width="70">
              <template slot-scope="{ row }">{{ row.nullable ? '是' : '否' }}</template>
            </el-table-column>
            <el-table-column prop="default" label="默认值" min-width="100"></el-table-column>
            <el-table-column prop="key" label="键" width="70"></el-table-column>
            <el-table-column prop="comment" label="注释" min-width="140"></el-table-column>
          </el-table>
        </div>
      </el-tab-pane>

      <el-tab-pane label="消息" name="message">
        <pre class="message-box">{{ message || '暂无消息' }}</pre>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script>
import { VxeTable, VxeColumn } from 'vxe-table';
import { downloadText, toCSV, toJSON } from '../lib/file';

export default {
  components: { VxeTable, VxeColumn },
  props: {
    result: { type: Object, default: null },
    structure: { type: Array, default: () => [] },
    message: { type: String, default: '' },
    loading: { type: Boolean, default: false },
    table: { type: Object, default: null },
  },
  data() {
    return {
      activeTab: 'data',
    };
  },
  computed: {
    columns() {
      return (this.result && this.result.columns) || [];
    },
    rows() {
      return (this.result && this.result.rows) || [];
    },
    isExecResult() {
      return !!(this.result && this.result.rowsAffected !== undefined);
    },
  },
  watch: {
    result() {
      if (this.result) {
        this.activeTab = 'data';
      }
    },
    message(value) {
      if (value && !this.result) {
        this.activeTab = 'message';
      }
    },
  },
  methods: {
    // Called by App while dragging the layout dividers so the virtual grid
    // re-measures its new height.
    recalculate() {
      if (this.$refs.grid && this.$refs.grid.recalculate) {
        this.$refs.grid.recalculate();
      }
    },
    exportCSV() {
      if (!this.columns.length) {
        return;
      }
      const name = this.table ? this.table.table : 'query';
      const csv = '\ufeff' + toCSV(this.columns.map((c) => c.name), this.rows);
      downloadText(csv, name + '.csv', 'text/csv;charset=utf-8');
    },
    exportJSON() {
      if (!this.columns.length) {
        return;
      }
      const name = this.table ? this.table.table : 'query';
      downloadText(
        toJSON(this.columns.map((c) => c.name), this.rows),
        name + '.json',
        'application/json;charset=utf-8',
      );
    },
  },
};
</script>
