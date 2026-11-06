<template>
  <div class="sqlui-app" :class="{ 'is-resizing': resizing }">
    <aside class="sqlui-sidebar" :style="{ width: sidebarWidth + 'px' }">
      <div class="sidebar-header">
        <i class="fa fa-database"></i>
        <span>SQL Console</span>
        <span class="version">{{ version }}</span>
      </div>
      <ConnectionTree :connections="connections" @select-table="openTable" />
      <div class="sidebar-footer">{{ connections.length }} 个连接</div>
    </aside>

    <div class="resize-divider vertical" @mousedown.prevent="startResize('sidebar', $event)"></div>

    <main class="sqlui-main">
      <div class="main-top" :style="{ height: consoleHeight + 'px' }">
        <SqlConsole
          :connection="activeConnection"
          :database="activeDatabase"
          :table="activeTable ? activeTable.table : ''"
          :hint-tables="hintTables"
          :loading="queryLoading"
          @execute="runSQL"
        />
      </div>

      <div class="resize-divider horizontal" @mousedown.prevent="startResize('console', $event)"></div>

      <div class="main-bottom">
        <ResultPanel
          ref="resultPanel"
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

const SIDEBAR_MIN = 180;
const SIDEBAR_MAX = 560;
const CONSOLE_MIN = 120;

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
      hintTables: {},
      message: '',
      queryLoading: false,
      sidebarWidth: 264,
      consoleHeight: 270,
      resizing: null,
    };
  },
  created() {
    try {
      const sw = parseInt(window.localStorage.getItem('sqlui_sidebar_width'), 10);
      const ch = parseInt(window.localStorage.getItem('sqlui_console_height'), 10);
      if (sw >= SIDEBAR_MIN && sw <= SIDEBAR_MAX) {
        this.sidebarWidth = sw;
      }
      if (ch >= CONSOLE_MIN) {
        this.consoleHeight = ch;
      }
    } catch (e) {
      // localStorage may be unavailable (private mode); ignore.
    }
  },
  mounted() {
    sqlClient.getConnections().then((list) => {
      this.connections = Array.isArray(list) ? list : [];
    }).catch((e) => {
      this.$message.error('加载连接失败: ' + e.message);
    });
  },
  methods: {
    clamp(value, min, max) {
      return Math.max(min, Math.min(max, value));
    },
    startResize(kind, e) {
      this.resizing = kind;
      this._resizeStartSize = kind === 'sidebar' ? this.sidebarWidth : this.consoleHeight;
      this._resizeStartX = e.clientX;
      this._resizeStartY = e.clientY;
      window.addEventListener('mousemove', this.onResizeMove);
      window.addEventListener('mouseup', this.stopResize);
      document.body.style.cursor = kind === 'sidebar' ? 'col-resize' : 'row-resize';
      document.body.style.userSelect = 'none';
    },
    onResizeMove(e) {
      if (!this.resizing) {
        return;
      }
      if (this.resizing === 'sidebar') {
        this.sidebarWidth = this.clamp(this._resizeStartSize + (e.clientX - this._resizeStartX), SIDEBAR_MIN, SIDEBAR_MAX);
      } else {
        this.consoleHeight = this.clamp(
          this._resizeStartSize + (e.clientY - this._resizeStartY),
          CONSOLE_MIN,
          window.innerHeight - 220,
        );
      }
      if (this.$refs.resultPanel && this.$refs.resultPanel.recalculate) {
        this.$refs.resultPanel.recalculate();
      }
    },
    stopResize() {
      if (!this.resizing) {
        return;
      }
      this.resizing = null;
      window.removeEventListener('mousemove', this.onResizeMove);
      window.removeEventListener('mouseup', this.stopResize);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
      try {
        window.localStorage.setItem('sqlui_sidebar_width', String(this.sidebarWidth));
        window.localStorage.setItem('sqlui_console_height', String(this.consoleHeight));
      } catch (e) {
        // ignore
      }
    },
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
        sqlClient.getTables(connection.id, database).catch(() => []),
      ]).then(([cols, data, keys, tableList]) => {
        this.structure = Array.isArray(cols) ? cols : [];
        this.result = data;
        this.primaryKeys = Array.isArray(keys) ? keys : [];
        this.message = '';
        this.queryLoading = false;

        // SQL autocomplete: all tables in the database (columns for the
        // currently selected table only, to avoid N metadata requests).
        const hint = {};
        (Array.isArray(tableList) ? tableList : []).forEach((t) => {
          hint[t] = null;
        });
        if (this.structure.length) {
          hint[table] = this.structure.map((c) => c.name);
        }
        this.hintTables = hint;
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
