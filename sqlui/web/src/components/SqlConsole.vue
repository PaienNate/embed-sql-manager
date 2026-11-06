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
        <span class="console-hint">Ctrl+Enter</span>
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

export default {
  props: {
    connection: { type: Object, default: null },
    database: { type: String, default: '' },
    loading: { type: Boolean, default: false },
  },
  data() {
    return {
      editor: null,
      ro: null,
    };
  },
  mounted() {
    this.editor = CodeMirror.fromTextArea(this.$refs.textarea, {
      mode: 'text/x-sql',
      lineNumbers: true,
      lineWrapping: true,
      indentUnit: 2,
      tabSize: 2,
      viewportMargin: 30,
      extraKeys: {
        'Ctrl-Enter': () => this.execute(),
        'Cmd-Enter': () => this.execute(),
      },
    });
    this.editor.setValue('SELECT 1;');
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
