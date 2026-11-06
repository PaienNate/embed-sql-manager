<template>
  <div class="tree">
    <div v-for="conn in connections" :key="conn.id" class="tree-conn">
      <div
        class="tree-row conn-row"
        :class="{ active: activeConnId === conn.id }"
        @click="toggleConnection(conn)">
        <i :class="['fa', isConnOpen(conn) ? 'fa-chevron-down' : 'fa-chevron-right']"></i>
        <i class="fa fa-server"></i>
        <span class="tree-label" :title="conn.name">{{ conn.name }}</span>
        <span v-if="conn.type" class="tree-badge">{{ conn.type }}</span>
        <i v-if="loading[conn.id]" class="fa fa-spinner fa-spin"></i>
      </div>
      <div v-if="isConnOpen(conn)" class="tree-children">
        <div v-for="db in databases[conn.id] || []" :key="db" class="tree-db">
          <div
            class="tree-row db-row"
            :class="{ active: activeDbKey === dbKey(conn, db) }"
            @click="toggleDatabase(conn, db)">
            <i :class="['fa', isDbOpen(conn, db) ? 'fa-chevron-down' : 'fa-chevron-right']"></i>
            <i class="fa fa-database"></i>
            <span class="tree-label" :title="db">{{ db || '(default)' }}</span>
            <i v-if="loading[dbKey(conn, db)]" class="fa fa-spinner fa-spin"></i>
          </div>
          <div v-if="isDbOpen(conn, db)" class="tree-tables">
            <div class="tree-row group-row" @click.stop="toggleGroup(conn, db)">
              <i :class="['fa', isGroupOpen(conn, db) ? 'fa-chevron-down' : 'fa-chevron-right']"></i>
              <i :class="['fa', isGroupOpen(conn, db) ? 'fa-folder-open' : 'fa-folder']"></i>
              <span class="tree-label">表</span>
              <span class="tree-count">{{ (tables[dbKey(conn, db)] || []).length }}</span>
            </div>
            <div v-if="isGroupOpen(conn, db)">
              <div
                v-for="table in tables[dbKey(conn, db)] || []"
                :key="table"
                class="tree-row table-row"
                :class="{ active: activeKey === tableKey(conn, db, table) }"
                @click="selectTable(conn, db, table)">
                <i class="fa fa-table"></i>
                <span class="tree-label" :title="table">{{ table }}</span>
              </div>
              <div v-if="!loading[dbKey(conn, db)] && !(tables[dbKey(conn, db)] || []).length" class="tree-empty table-empty">没有表</div>
            </div>
          </div>
        </div>
        <div v-if="!loading[conn.id] && !(databases[conn.id] || []).length" class="tree-empty">没有数据库</div>
      </div>
    </div>
    <div v-if="!connections.length" class="tree-empty">没有注册连接</div>
  </div>
</template>

<script>
import * as sqlClient from '../sqlClient';

export default {
  props: {
    connections: {
      type: Array,
      default: () => [],
    },
  },
  data() {
    return {
      expandedConn: {},
      expandedDb: {},
      expandedGroup: {},
      databases: {},
      tables: {},
      loading: {},
      activeConnId: '',
      activeDbKey: '',
      activeKey: '',
    };
  },
  methods: {
    isConnOpen(conn) {
      return !!this.expandedConn[conn.id];
    },
    isDbOpen(conn, db) {
      return !!this.expandedDb[this.dbKey(conn, db)];
    },
    isGroupOpen(conn, db) {
      const v = this.expandedGroup[this.dbKey(conn, db)];
      return v === undefined ? true : !!v;
    },
    dbKey(conn, db) {
      return conn.id + '|' + db;
    },
    tableKey(conn, db, table) {
      return conn.id + '|' + db + '|' + table;
    },
    maybeSelectDefaultDatabase(conn) {
      const dbs = this.databases[conn.id] || [];
      if (!dbs.length) {
        return;
      }
      // Auto-open the first database/schema so completion has metadata right
      // after selecting the connection. PostgreSQL GetDatabases sorts
      // current_schema() (the DSN search_path) first.
      const db = dbs[0];
      this.$set(this.expandedDb, this.dbKey(conn, db), true);
      this.$emit('select-database', { connection: conn, database: db });
      this.loadTables(conn, db);
    },
    loadTables(conn, db) {
      const key = this.dbKey(conn, db);
      if (this.tables[key]) {
        this.$emit('tables-loaded', {
          connection: conn,
          database: db,
          tables: this.tables[key],
        });
        return;
      }
      if (this.loading[key]) {
        return;
      }
      this.$set(this.loading, key, true);
      sqlClient.getTables(conn.id, db).then((list) => {
        const tables = Array.isArray(list) ? list : [];
        this.$set(this.tables, key, tables);
        this.$set(this.loading, key, false);
        this.$emit('tables-loaded', { connection: conn, database: db, tables });
      }).catch((e) => {
        this.$set(this.loading, key, false);
        this.$message.error(e.message);
      });
    },
    toggleConnection(conn) {
      const open = this.isConnOpen(conn);
      this.$set(this.expandedConn, conn.id, !open);
      this.activeConnId = conn.id;
      this.activeDbKey = '';
      this.$emit('select-connection', { connection: conn });
      if (this.databases[conn.id]) {
        this.maybeSelectDefaultDatabase(conn);
        return;
      }
      if (!this.loading[conn.id]) {
        this.$set(this.loading, conn.id, true);
        sqlClient.getDatabases(conn.id).then((dbs) => {
          const list = Array.isArray(dbs) ? dbs : [];
          this.$set(this.databases, conn.id, list);
          this.$set(this.loading, conn.id, false);
          this.maybeSelectDefaultDatabase(conn);
        }).catch((e) => {
          this.$set(this.loading, conn.id, false);
          this.$message.error(e.message);
        });
      }
    },
    toggleDatabase(conn, db) {
      const key = this.dbKey(conn, db);
      const open = this.isDbOpen(conn, db);
      this.$set(this.expandedDb, key, !open);
      this.activeConnId = conn.id;
      this.activeDbKey = key;
      this.$emit('select-database', { connection: conn, database: db });
      this.loadTables(conn, db);
    },
    toggleGroup(conn, db) {
      const key = this.dbKey(conn, db);
      this.$set(this.expandedGroup, key, !this.isGroupOpen(conn, db));
    },
    selectTable(conn, db, table) {
      this.activeConnId = conn.id;
      this.activeDbKey = this.dbKey(conn, db);
      this.activeKey = this.tableKey(conn, db, table);
      this.$emit('select-table', { connection: conn, database: db, table });
    },
  },
};
</script>
