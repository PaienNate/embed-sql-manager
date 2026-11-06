<template>
  <div v-loading="loading" class="result-panel">
    <el-tabs v-model="activeTab" class="result-tabs">
      <el-tab-pane label="数据" name="data">
        <div class="result-pane">
          <div v-if="table" class="result-toolbar">
            <span class="result-table-name"><i class="fa fa-table"></i>{{ table.table }}</span>
            <span v-if="result && result.truncated" class="result-warn">仅显示前 {{ rows.length }} 行</span>
            <span class="spacer"></span>
            <el-button v-if="canInsert" size="mini" @click="openInsert">
              <i class="fa fa-plus"></i> 新增行
            </el-button>
            <el-button v-if="table" size="mini" @click="$emit('refresh')">
              <i class="fa fa-refresh"></i> 刷新
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
              <vxe-column v-if="canEdit" title="操作" width="120" fixed="right">
                <template #default="{ row, rowIndex }">
                  <el-button type="text" size="mini" @click.stop="openEdit(row, rowIndex)">编辑</el-button>
                  <el-button type="text" size="mini" class="danger-text" @click.stop="removeRow(row)">删除</el-button>
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

    <el-dialog
      :title="dialogMode === 'insert' ? '新增行' : '编辑行'"
      :visible.sync="dialogVisible"
      width="680px"
      append-to-body>
      <div class="dialog-fields">
        <el-form label-width="150px" size="mini">
          <el-form-item v-for="f in dialogFields" :key="f.name" :label="f.name">
            <div class="dialog-field-row">
              <el-checkbox v-model="f.isNull" :disabled="f.disabled" @change="f.touched = true">NULL</el-checkbox>
              <el-input v-model="f.value" :disabled="f.isNull || f.disabled" @input="f.touched = true"></el-input>
              <span class="field-type">{{ f.type }}</span>
            </div>
          </el-form-item>
        </el-form>
      </div>
      <span slot="footer">
        <el-button size="mini" @click="dialogVisible = false">取消</el-button>
        <el-button size="mini" type="primary" :loading="saving" @click="saveDialog">保存</el-button>
      </span>
    </el-dialog>
  </div>
</template>

<script>
import { VxeTable, VxeColumn } from 'vxe-table';
import * as sqlClient from '../sqlClient';

export default {
  components: { VxeTable, VxeColumn },
  props: {
    result: { type: Object, default: null },
    structure: { type: Array, default: () => [] },
    message: { type: String, default: '' },
    loading: { type: Boolean, default: false },
    table: { type: Object, default: null },
    capabilities: { type: Object, default: () => ({ query: true, exec: false, editing: false }) },
    primaryKeys: { type: Array, default: () => [] },
  },
  data() {
    return {
      activeTab: 'data',
      dialogVisible: false,
      dialogMode: 'update',
      dialogFields: [],
      dialogOriginal: null,
      dialogRowIndex: -1,
      saving: false,
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
    canInsert() {
      return !!(this.table && this.capabilities && this.capabilities.editing);
    },
    canEdit() {
      return !!(this.table && this.capabilities && this.capabilities.editing &&
        this.primaryKeys && this.primaryKeys.length);
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
    fieldFromColumn(col, value, disabled) {
      const isNull = value === null || value === undefined;
      return {
        name: col.name,
        type: col.type || '',
        value: isNull ? '' : String(value),
        isNull,
        touched: false,
        disabled: !!disabled,
      };
    },
    openInsert() {
      const cols = this.columns.length ? this.columns : this.structure;
      if (!cols.length) {
        this.$message.warning('没有可用的列信息');
        return;
      }
      this.dialogMode = 'insert';
      this.dialogOriginal = null;
      this.dialogRowIndex = -1;
      this.dialogFields = cols.map((c) => this.fieldFromColumn(c, null, false));
      this.dialogVisible = true;
    },
    openEdit(row, rowIndex) {
      if (!this.columns.length) {
        return;
      }
      this.dialogMode = 'update';
      this.dialogOriginal = row.slice();
      this.dialogRowIndex = rowIndex;
      this.dialogFields = this.columns.map((c, i) => {
        const isPK = this.primaryKeys.indexOf(c.name) >= 0;
        return this.fieldFromColumn(c, row[i], isPK);
      });
      this.dialogVisible = true;
    },
    buildPK(row) {
      const pk = {};
      this.primaryKeys.forEach((name) => {
        const idx = this.columns.findIndex((c) => c.name === name);
        if (idx >= 0) {
          pk[name] = row[idx];
        }
      });
      return pk;
    },
    saveDialog() {
      if (!this.table) {
        return;
      }
      const connID = this.table.connection.id;
      const database = this.table.database || '';
      const table = this.table.table;

      if (this.dialogMode === 'insert') {
        const values = {};
        let count = 0;
        this.dialogFields.forEach((f) => {
          if (!f.touched) {
            return;
          }
          values[f.name] = f.isNull ? null : f.value;
          count += 1;
        });
        if (!count) {
          this.$message.warning('请至少填写一个字段');
          return;
        }
        this.saving = true;
        sqlClient.insertRow(connID, database, table, values).then(() => {
          this.saving = false;
          this.dialogVisible = false;
          this.$message.success('新增成功');
          this.$emit('row-saved');
        }).catch((e) => {
          this.saving = false;
          this.$message.error(e.message);
        });
        return;
      }

      const pk = this.buildPK(this.dialogOriginal);
      if (!Object.keys(pk).length) {
        this.$message.warning('该表没有主键，无法安全更新');
        return;
      }
      const changes = {};
      let count = 0;
      this.dialogFields.forEach((f, i) => {
        if (f.disabled || !f.touched) {
          return;
        }
        const original = this.dialogOriginal[i];
        const originalNull = original === null || original === undefined;
        if (f.isNull !== originalNull) {
          changes[f.name] = f.isNull ? null : f.value;
          count += 1;
        } else if (!f.isNull && String(original) !== f.value) {
          changes[f.name] = f.value;
          count += 1;
        }
      });
      if (!count) {
        this.dialogVisible = false;
        return;
      }
      this.saving = true;
      sqlClient.updateRow(connID, database, table, pk, changes).then((res) => {
        this.saving = false;
        this.dialogVisible = false;
        if (res && res.rowsAffected === 0) {
          this.$message.warning('没有行被更新（可能已被修改）');
        } else {
          this.$message.success('更新成功');
        }
        this.$emit('row-saved');
      }).catch((e) => {
        this.saving = false;
        this.$message.error(e.message);
      });
    },
    removeRow(row) {
      if (!this.table || !this.canEdit) {
        return;
      }
      const pk = this.buildPK(row);
      if (!Object.keys(pk).length) {
        this.$message.warning('该表没有主键，无法安全删除');
        return;
      }
      this.$confirm('确定删除这一行？', '确认删除', { type: 'warning' }).then(() => {
        const connID = this.table.connection.id;
        const database = this.table.database || '';
        return sqlClient.deleteRow(connID, database, this.table.table, pk);
      }).then((res) => {
        if (res && res.rowsAffected === 0) {
          this.$message.warning('没有行被删除（可能已被修改）');
        } else {
          this.$message.success('删除成功');
        }
        this.$emit('row-saved');
      }).catch((e) => {
        if (e && e !== 'cancel' && e !== 'close') {
          this.$message.error(e.message || String(e));
        }
      });
    },
  },
};
</script>
