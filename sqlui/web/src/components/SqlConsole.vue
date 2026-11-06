<template>
  <div class="sql-console">
    <div class="console-toolbar">
      <div class="console-title">
        <i class="fa fa-terminal"></i>
        <span>SQL 控制台</span>
        <span v-if="connection" class="console-conn">
          {{ connection.name }}<template v-if="database"> / {{ database }}</template>
        </span>
        <span v-else class="console-conn">未选择连接</span>
      </div>
      <div class="console-actions">
        <el-button
          type="primary"
          size="mini"
          :loading="loading"
          :disabled="!connection"
          @click="execute">
          <i class="fa fa-play"></i> 执行
        </el-button>
        <span class="console-hint">Ctrl+Enter 执行 · Ctrl+Space 补全</span>
      </div>
    </div>
    <div class="console-editor">
      <textarea ref="textarea"></textarea>
    </div>
  </div>
</template>

<script>
import CodeMirror from 'codemirror';
import 'codemirror/lib/codemirror.css';
import 'codemirror/mode/sql/sql.js';
import 'codemirror/addon/hint/show-hint';
import 'codemirror/addon/hint/show-hint.css';
import 'codemirror/addon/hint/sql-hint';

export default {
  props: {
    value: { type: String, default: '' },
    connection: { type: Object, default: null },
    database: { type: String, default: '' },
    table: { type: String, default: '' },
    hintTables: { type: Object, default: () => ({}) },
    loading: { type: Boolean, default: false },
  },
  data() {
    return {
      editor: null,
      ro: null,
    };
  },
  watch: {
    hintTables: {
      handler() {
        this.applyHintOptions();
      },
      deep: true,
    },
    table() {
      this.applyHintOptions();
    },
    value(val) {
      if (!this.editor) {
        return;
      }
      if (this.editor.getValue() === (val || '')) {
        return;
      }
      this._syncing = true;
      this.editor.setValue(val || '');
      this._syncing = false;
    },
  },
  mounted() {
    this.editor = CodeMirror.fromTextArea(this.$refs.textarea, {
      mode: 'text/x-sql',
      lineNumbers: true,
      lineWrapping: true,
      indentUnit: 2,
      tabSize: 2,
      viewportMargin: 30,
      hintOptions: {
        completeSingle: false,
        alignWithWord: true,
        tables: this.hintTables || {},
        defaultTable: this.table || undefined,
      },
      extraKeys: {
        'Ctrl-Enter': () => this.execute(),
        'Cmd-Enter': () => this.execute(),
        'Ctrl-Space': 'autocomplete',
        'Cmd-Space': 'autocomplete',
      },
    });
    this._syncing = false;
    this.editor.setValue(this.value || 'SELECT 1;');

    // Show hints while typing a word (not on every keystroke of a symbol).
    this.editor.on('change', (cm) => {
      if (this._syncing) {
        return;
      }
      this.$emit('change', cm.getValue());
      if (cm.state.completionActive) {
        return;
      }
      const cursor = cm.getCursor();
      const token = cm.getTokenAt(cursor);
      if (token && token.string && /^[\w"`]+$/.test(token.string)) {
        cm.showHint({ completeSingle: false });
      }
    });

    if (typeof ResizeObserver !== 'undefined') {
      this.ro = new ResizeObserver(() => this.refresh());
      this.ro.observe(this.$el.querySelector('.console-editor'));
    }
    window.addEventListener('resize', this.refresh);
    this.$nextTick(this.refresh);
  },
  beforeDestroy() {
    window.removeEventListener('resize', this.refresh);
    if (this.ro) {
      this.ro.disconnect();
      this.ro = null;
    }
    if (this.editor) {
      this.editor.toTextArea();
      this.editor = null;
    }
  },
  methods: {
    refresh() {
      if (this.editor) {
        this.editor.refresh();
      }
    },
    applyHintOptions() {
      if (!this.editor) {
        return;
      }
      this.editor.setOption('hintOptions', {
        completeSingle: false,
        alignWithWord: true,
        tables: this.hintTables || {},
        defaultTable: this.table || undefined,
      });
    },
    execute() {
      if (!this.editor || !this.connection) {
        return;
      }
      const sql = this.editor.getValue().trim();
      if (!sql) {
        this.$message.warning('请输入 SQL');
        return;
      }
      this.$emit('execute', sql);
    },
  },
};
</script>
