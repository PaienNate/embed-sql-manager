<template>
  <div class="sqlui-app" :class="{ 'is-resizing': resizing }">
    <aside class="sqlui-sidebar" :style="{ width: sidebarWidth + 'px' }">
      <div class="sidebar-header">
        <i class="fa fa-database"></i>
        <span>SQL Console</span>
        <span class="version">{{ version }}</span>
      </div>
      <ConnectionTree
        :connections="connections"
        @select-table="openTable"
        @select-connection="onSelectConnection"
        @select-database="onSelectDatabase"
        @tables-loaded="onTablesLoaded"
      />
      <div class="sidebar-footer">{{ connections.length }} 个连接</div>
    </aside>

    <div class="resize-divider vertical" @mousedown.prevent="startResize('sidebar', $event)"></div>

    <main class="sqlui-main">
      <div class="main-top" :style="{ height: consoleHeight + 'px' }">
        <QueryTabs
          :tabs="queryTabs"
          :active-id="activeTabId"
          @select="switchTab"
          @add="addTab"
          @close="closeTab"
          @import="pickImportSQL"
          @export="exportActiveSQL"
          @export-all="exportAllSQL"
        />
        <SqlConsole
          :value="sql"
          :connection="activeConnection"
          :database="activeDatabase"
          :table="activeTable ? activeTable.table : ''"
          :hint-tables="hintTables"
          :loading="queryLoading"
          @change="onSqlChange"
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
          @refresh="reloadTable"
        />
      </div>

      <footer class="sqlui-statusbar">
        <span v-if="activeConnection">
          <span class="status-tab">{{ activeTabTitle }}</span>
          <span class="status-sep">·</span>
          {{ activeConnection.name }}
          <template v-if="activeDatabase"> / {{ activeDatabase }}</template>
          <template v-if="activeTable"> / {{ activeTable.table }}</template>
        </span>
        <span v-else>请选择左侧的数据源，或在 SQL 控制台执行查询</span>
        <span class="status-right">
          <span v-if="result && result.rowsAffected === undefined && result.rowCount !== undefined">{{ result.rowCount }} 行</span>
          <span v-if="result && result.elapsedMs !== undefined"> · {{ result.elapsedMs }} ms</span>
          <span v-if="result && result.truncated"> · 已截断</span>
          <span class="version">{{ version }}</span>
        </span>
      </footer>
    </main>

    <input
      ref="sqlFile"
      type="file"
      accept=".sql,.txt,text/plain"
      style="display:none"
      @change="onSQLFileChange">
  </div>
</template>

<script>
import ConnectionTree from './components/ConnectionTree';
import QueryTabs from './components/QueryTabs';
import SqlConsole from './components/SqlConsole';
import ResultPanel from './components/ResultPanel';
import * as sqlClient from './sqlClient';
import { downloadText, readFileText } from './lib/file';

const SIDEBAR_MIN = 180;
const SIDEBAR_MAX = 560;
const CONSOLE_MIN = 120;

export default {
  components: { ConnectionTree, QueryTabs, SqlConsole, ResultPanel },
  data() {
    return {
      version: (typeof window !== 'undefined' && window.__SQLUI__ && window.__SQLUI__.version) || '',
      connections: [],
      sql: 'SELECT 1;',
      activeConnection: null,
      activeDatabase: '',
      activeTable: null,
      result: null,
      structure: [],
      hintTables: {},
      message: '',
      queryLoading: false,
      sidebarWidth: 264,
      consoleHeight: 270,
      resizing: null,
      queryTabs: [],
      activeTabId: '',
      tabSeq: 1,
    };
  },
  computed: {
    activeTab() {
      for (let i = 0; i < this.queryTabs.length; i++) {
        if (this.queryTabs[i].id === this.activeTabId) {
          return this.queryTabs[i];
        }
      }
      return null;
    },
    activeTabTitle() {
      return this.activeTab ? this.activeTab.title : '';
    },
  },
  created() {
    const savedTabs = this.readSavedTabs();
    if (savedTabs.length) {
      this.queryTabs = savedTabs;
      this.tabSeq = savedTabs.length + 1;
      let savedActive = '';
      try {
        savedActive = window.localStorage.getItem('sqlui_active_tab') || '';
      } catch (e) {
        // ignore
      }
      let found = false;
      for (let i = 0; i < savedTabs.length; i++) {
        if (savedTabs[i].id === savedActive) {
          found = true;
          break;
        }
      }
      this.activeTabId = found ? savedActive : savedTabs[0].id;
    } else {
      const tab = this.makeTab('SELECT 1;', '查询 1');
      this.queryTabs = [tab];
      this.activeTabId = tab.id;
    }
    this.loadActiveTab();

    try {
      const sw = parseInt(window.localStorage.getItem('sqlui_sidebar_width'), 10);
      const ch = parseInt(window.localStorage.getItem('sqlui_console_height'), 10);
      if (sw >= SIDEBAR_MIN && sw <= SIDEBAR_MAX) {
        this.sidebarWidth = sw;
      }
      if (ch >= CONSOLE_MIN) {
        this.consoleHeight = ch;
      } else {
        // Default: SQL console takes 3/4 of the main area, results 1/4.
        const available = (window.innerHeight || 900) - 34; // statusbar + dividers
        this.consoleHeight = Math.max(CONSOLE_MIN, Math.round(available * 0.75));
      }
    } catch (e) {
      // localStorage may be unavailable (private mode); ignore.
    }
    this.persistTabs();
  },
  beforeDestroy() {
    if (this._persistTimer) {
      clearTimeout(this._persistTimer);
      this._persistTimer = null;
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
    readSavedTabs() {
      try {
        const raw = window.localStorage.getItem('sqlui_query_tabs');
        if (!raw) {
          return [];
        }
        const arr = JSON.parse(raw);
        if (!Array.isArray(arr)) {
          return [];
        }
        const out = [];
        for (let i = 0; i < arr.length; i++) {
          const t = arr[i];
          if (!t || !t.id) {
            continue;
          }
          out.push({
            id: t.id,
            title: t.title || ('查询 ' + (i + 1)),
            sql: typeof t.sql === 'string' ? t.sql : '',
            connection: null,
            database: '',
            activeTable: null,
            result: null,
            structure: [],
            message: '',
            hintTables: {},
          });
        }
        return out;
      } catch (e) {
        return [];
      }
    },
    makeTab(sql, title) {
      return {
        id: 'q' + this.tabSeq++,
        title: title || ('查询 ' + (this.tabSeq - 1)),
        sql: typeof sql === 'string' ? sql : 'SELECT 1;',
        connection: null,
        database: '',
        activeTable: null,
        result: null,
        structure: [],
        message: '',
        hintTables: {},
      };
    },
    loadActiveTab() {
      const t = this.activeTab;
      if (!t) {
        return;
      }
      this.sql = t.sql || '';
      this.activeConnection = t.connection;
      this.activeDatabase = t.database || '';
      this.activeTable = t.activeTable;
      this.result = t.result;
      this.structure = t.structure || [];
      this.hintTables = t.hintTables || {};
      this.message = t.message || '';
    },
    saveActiveTab() {
      const t = this.activeTab;
      if (!t) {
        return;
      }
      t.sql = this.sql;
      t.connection = this.activeConnection;
      t.database = this.activeDatabase;
      t.activeTable = this.activeTable;
      t.result = this.result;
      t.structure = this.structure;
      t.hintTables = this.hintTables;
      t.message = this.message;
    },
    persistTabs() {
      if (this._persistTimer) {
        clearTimeout(this._persistTimer);
      }
      this._persistTimer = setTimeout(() => {
        this._persistTimer = null;
        try {
          const data = this.queryTabs.map((t) => ({
            id: t.id,
            title: t.title,
            sql: t.sql,
          }));
          window.localStorage.setItem('sqlui_query_tabs', JSON.stringify(data));
          window.localStorage.setItem('sqlui_active_tab', this.activeTabId);
        } catch (e) {
          // ignore
        }
      }, 300);
    },
    switchTab(id) {
      if (id === this.activeTabId) {
        return;
      }
      this.saveActiveTab();
      this.activeTabId = id;
      this.loadActiveTab();
      this.persistTabs();
    },
    addTab(sql, title) {
      this.saveActiveTab();
      const tab = this.makeTab(typeof sql === 'string' ? sql : 'SELECT 1;', title);
      // Navicat-style: a new query tab inherits the current data source.
      tab.connection = this.activeConnection;
      tab.database = this.activeDatabase;
      tab.hintTables = Object.assign({}, this.hintTables);
      this.queryTabs.push(tab);
      this.activeTabId = tab.id;
      this.loadActiveTab();
      this.persistTabs();
    },
    closeTab(id) {
      if (this.queryTabs.length <= 1) {
        return;
      }
      this.saveActiveTab();
      let idx = -1;
      for (let i = 0; i < this.queryTabs.length; i++) {
        if (this.queryTabs[i].id === id) {
          idx = i;
          break;
        }
      }
      if (idx < 0) {
        return;
      }
      this.queryTabs.splice(idx, 1);
      if (this.activeTabId === id) {
        const next = this.queryTabs[Math.max(0, idx - 1)] || this.queryTabs[0];
        this.activeTabId = next.id;
        this.loadActiveTab();
      }
      this.persistTabs();
    },
    onSqlChange(value) {
      this.sql = value;
      const t = this.activeTab;
      if (t) {
        t.sql = value;
      }
      this.persistTabs();
    },
    onSelectConnection(payload) {
      if (!payload || !payload.connection) {
        return;
      }
      this.activeConnection = payload.connection;
      this.activeDatabase = '';
      this.activeTable = null;
      this.hintTables = {};
      this.saveActiveTab();
    },
    onSelectDatabase(payload) {
      if (!payload || !payload.connection) {
        return;
      }
      this.activeConnection = payload.connection;
      this.activeDatabase = payload.database || '';
      this.activeTable = null;
      this.saveActiveTab();
    },
    onTablesLoaded(payload) {
      if (!payload || !payload.tables) {
        return;
      }
      const hint = {};
      for (let i = 0; i < payload.tables.length; i++) {
        hint[payload.tables[i]] = [];
      }
      const existing = this.hintTables || {};
      Object.keys(existing).forEach((k) => {
        if (Array.isArray(existing[k]) && existing[k].length && hint[k] && hint[k].length === 0) {
          hint[k] = existing[k];
        }
      });
      this.hintTables = hint;
      this.saveActiveTab();
    },
    openTable(payload) {
      const connection = payload.connection;
      const table = payload.table;
      const database = payload.database || '';
      this.saveActiveTab();
      this.activeConnection = connection;
      this.activeDatabase = database;
      this.activeTable = { connection, database, table };
      this.queryLoading = true;
      Promise.all([
        sqlClient.getColumns(connection.id, database, table),
        sqlClient.getTableData(connection.id, database, table, 1000),
        sqlClient.getTables(connection.id, database).catch(() => []),
      ]).then(([cols, data, tableList]) => {
        this.structure = Array.isArray(cols) ? cols : [];
        this.result = data;
        this.message = '';
        this.queryLoading = false;

        const hint = {};
        (Array.isArray(tableList) ? tableList : []).forEach((t) => {
          hint[t] = [];
        });
        const existing = this.hintTables || {};
        Object.keys(existing).forEach((k) => {
          if (Array.isArray(existing[k]) && existing[k].length && hint[k] && hint[k].length === 0) {
            hint[k] = existing[k];
          }
        });
        if (this.structure.length) {
          hint[table] = this.structure.map((c) => c.name);
        }
        this.hintTables = hint;
        this.saveActiveTab();
      }).catch((e) => {
        this.queryLoading = false;
        this.structure = [];
        this.result = null;
        this.message = e.message;
        this.$message.error(e.message);
        this.saveActiveTab();
      });
    },
    runSQL(sql) {
      if (!this.activeConnection) {
        this.$message.warning('请先在左侧选择一个数据源');
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
        this.saveActiveTab();
      }).catch((e) => {
        this.queryLoading = false;
        this.result = null;
        this.activeTable = null;
        this.structure = [];
        this.message = e.message;
        this.$message.error(e.message);
        this.saveActiveTab();
      });
    },
    reloadTable() {
      if (this.activeTable) {
        this.openTable(this.activeTable);
      }
    },
    pickImportSQL() {
      if (!this.$refs.sqlFile) {
        return;
      }
      this.$refs.sqlFile.value = '';
      this.$refs.sqlFile.click();
    },
    onSQLFileChange(e) {
      const file = e.target.files && e.target.files[0];
      if (!file) {
        return;
      }
      readFileText(file).then((text) => {
        const title = file.name.replace(/\.(sql|txt)$/i, '') || ('导入 ' + (this.queryTabs.length + 1));
        this.addTab(text, title);
        this.$message.success('已导入 SQL');
      }).catch((err) => {
        this.$message.error(err.message || String(err));
      });
    },
    exportActiveSQL() {
      this.saveActiveTab();
      const t = this.activeTab;
      if (!t) {
        return;
      }
      downloadText((t.sql || '') + '\n', (t.title || 'query') + '.sql', 'text/plain;charset=utf-8');
    },
    exportAllSQL() {
      this.saveActiveTab();
      const parts = this.queryTabs.map((t) => '-- ===== ' + t.title + ' =====\n' + (t.sql || ''));
      downloadText(parts.join('\n\n'), 'queries.sql', 'text/plain;charset=utf-8');
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
          window.innerHeight - 140,
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
  },
};
</script>
