<template>
  <div class="sqlui-app">
    <aside class="sqlui-sidebar">
      <div class="sidebar-header">
        <i class="fa fa-database"></i>
        <span>SQL Console</span>
        <span class="version">{{ version }}</span>
      </div>
      <ConnectionTree :connections="connections" @select-table="openTable" />
      <div class="sidebar-footer">{{ connections.length }} 个连接</div>
    </aside>

    <main class="sqlui-main">
      <div class="main-top">
        <SqlConsole
          :connection="activeConnection"
          :database="activeDatabase"
          :loading="queryLoading"
          @execute="runSQL"
        />
      </div>
      <div class="main-bottom">
        <ResultPanel
          :result="result"
          :structure="structure"
          :message="message"
          :loading="queryLoading"
          :table="activeTable"
          :capabilities="activeCaps"
          :primary-keys="primaryKeys"
          @refresh="reloadTable"
          @row-saved="reloadTable"
        />
      </div>
      <footer class="sqlui-statusbar">
        <span v-if="activeConnection">
          {{ activeConnection.name }}
          <template v-if="activeDatabase"> / {{ activeDatabase }}</template>
          <template v-if="activeTable"> / {{ activeTable.table }}</template>
        </span>
        <span v-else>请选择左侧的表，或在 SQL 控制台执行查询</span>
        <span class="status-right">
          <span v-if="result && result.rowsAffected === undefined && result.rowCount !== undefined">{{ result.rowCount }} 行</span>
          <span v-if="result && result.elapsedMs !== undefined"> · {{ result.elapsedMs }} ms</span>
          <span v-if="result && result.truncated"> · 已截断</span>
          <span class="version">{{ version }}</span>
        </span>
      </footer>
    </main>
  </div>
</template>

<script>
import ConnectionTree from './components/ConnectionTree';
import SqlConsole from './components/SqlConsole';
import ResultPanel from './components/ResultPanel';
import * as sqlClient from './sqlClient';

export default {
  components: { ConnectionTree, SqlConsole, ResultPanel },
  data() {
    return {
      version: (typeof window !== 'undefined' && window.__SQLUI__ && window.__SQLUI__.version) || '',
      connections: [],
      activeConnection: null,
      activeDatabase: '',
      activeTable: null,
      activeCaps: { query: true, exec: false, editing: false },
      result: null,
      structure: [],
      primaryKeys: [],
      message: '',
      queryLoading: false,
    };
  },
  mounted() {
    sqlClient.getConnections().then((list) => {
      this.connections = Array.isArray(list) ? list : [];
    }).catch((e) => {
      this.$message.error('加载连接失败: ' + e.message);
    });
  },
  methods: {
    openTable(payload) {
      const { connection, table } = payload;
      const database = payload.database || '';
      this.activeConnection = connection;
      this.activeDatabase = database;
      this.activeTable = { connection, database, table };
      this.activeCaps = connection.capabilities || { query: true, exec: false, editing: false };
      this.queryLoading = true;
      Promise.all([
        sqlClient.getColumns(connection.id, database, table),
        sqlClient.getTableData(connection.id, database, table, 1000),
        sqlClient.getPrimaryKeys(connection.id, database, table).catch(() => []),
      ]).then(([cols, data, keys]) => {
        this.structure = Array.isArray(cols) ? cols : [];
        this.result = data;
        this.primaryKeys = Array.isArray(keys) ? keys : [];
        this.message = '';
        this.queryLoading = false;
      }).catch((e) => {
        this.queryLoading = false;
        this.structure = [];
        this.primaryKeys = [];
        this.result = null;
        this.message = e.message;
        this.$message.error(e.message);
      });
    },
    runSQL(sql) {
      if (!this.activeConnection) {
        this.$message.warning('请先在左侧选择一个连接');
        return;
      }
      if (!sql || !sql.trim()) {
        return;
      }
      this.queryLoading = true;
      sqlClient.query(this.activeConnection.id, sql).then((res) => {
        this.queryLoading = false;
        if (res && res.rowsAffected !== undefined) {
          this.result = res;
          this.message = '执行成功，影响 ' + res.rowsAffected + ' 行' +
            (res.lastInsertId ? '，自增 ID: ' + res.lastInsertId : '');
          this.$message.success('执行成功');
        } else {
          this.result = res;
          this.message = '';
        }
        this.activeTable = null;
        this.structure = [];
        this.primaryKeys = [];
      }).catch((e) => {
        this.queryLoading = false;
        this.result = null;
        this.activeTable = null;
        this.structure = [];
        this.primaryKeys = [];
        this.message = e.message;
        this.$message.error(e.message);
      });
    },
    reloadTable() {
      if (this.activeTable) {
        this.openTable(this.activeTable);
      }
    },
  },
};
</script>
