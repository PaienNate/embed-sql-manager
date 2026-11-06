# AGENTS.md

面向在本仓库工作的 AI 代理 / 开发者的约定。开始改动前先读本文件。

## 项目定位

可内嵌的 SQL 控制台库：宿主实现 `Provider`（可选 `ExecProvider`），`sqlui.Register` 在 `http.ServeMux` 上挂载 Vue2 界面和 API。**核心包不依赖 GORM、SQL 方言实现或任何数据库驱动**，只用标准库。

## 目录

- `sqlui/` —— 库核心：
  - `provider.go`：`Provider` / `ExecProvider` 接口与 DTO。
  - `sqlui.go`：`Config` / `Connection` / `Register` / 前端注入 / 前缀。
  - `handlers.go`：连接、库/表/列、表数据、查询端点。
  - `rows.go`：`*sql.Rows` → JSON（ColumnTypes、NULL、二进制、1000 行截断）。
  - `query.go`：查询/执行语句的简单分类。
  - `web/`：Vue2 前端源码与 `dist/`（**构建产物必须提交**）。
- `example/`：SQLite + PostgreSQL Provider 参考实现（`database/sql`）。
  - `provider.go`：SQLite（`mattn/go-sqlite3`）。
  - `postgres.go`：PostgreSQL（`github.com/lib/pq`，元数据 SQL 参考 `sys_api_service`）。

## 构建与测试

```bash
go build ./... && go test ./...
gofmt -l sqlui/ example/          # 必须为空
cd sqlui/web && npm install && npm run subset-icons && npm run build
```

## 关键约定

1. **核心不 import 驱动/GORM/sqlex**：数据库差异由宿主的 `Provider` 实现。`go.mod` 里的 `mattn/go-sqlite3`、`github.com/lib/pq` 只服务 `example/`。
2. **`dist/` 提交进仓库**：`//go:embed all:web/dist` 在编译期读取；改前端后必须重建 dist 并提交。
3. **`ExecuteSQL(ctx, sql)` 签名**与 `sys_api_service` 的 `QueryExecutor` 一致；SQL 控制台按数据源当前库执行，树里的 database 只影响元数据和 `SelectTable`。
4. **`SelectTable` 必须实现**（表预览）；`ExecProvider` 可选，未实现时 SQL 控制台只读。
5. **不做行级编辑**：数据网格只读；增删改用 SQL 控制台写语句，避免各库主键/占位符/类型差异。
6. **`MaxRows` 默认 1000**：`readRows` 读满即 `truncated`；不要把大结果集一次性塞进内存。
7. **不暴露凭据**：核心接口不传 DSN/password；`Connection.Type` 只用于显示。`example/main.go` 里按需求硬编码了 `datacenterDB` 的演示凭据，**只用于本地 example，不要复用到其他环境**。
8. **IE11**：Vite legacy 构建；避免 CSS 变量作为关键布局、flex `gap` 作为关键间距；图标走 `scripts/subset-fontawesome.js` 子集流程。
9. **SQL 补全**：`hintTables` 的值必须是数组（未知列用 `[]`，不能用 `null`）——CodeMirror `sql-hint` 的 `wrapTable` 会对 `null` 读 `.text` 直接报错。新建查询页签继承当前 `connection/database/hintTables`。
10. **页签/布局持久化**：`sqlui_query_tabs` / `sqlui_active_tab` 存页签 SQL，`sqlui_sidebar_width` / `sqlui_console_height` 存布局；结果集不持久化。结果导出/查询页签导入导出是纯前端行为，后端端点不变。不做“数据导入到表”（容易误解为写库）。
11. **PostgreSQL 示例**：`GetDatabases` 返回 **schema**（不是集群 database），并把 `current_schema()`（DSN `search_path` 的第一个）排在最前；`ConnectionTree` 会自动展开第一个库/schema 并加载表。SQL 控制台按 DSN 的 `search_path` 执行，表数据预览按选中的 schema 限定表名。`strictCreateMode` 是 GORM 的迁移保护，不在本库生效，控制台能力由数据库账号权限决定。
12. **左侧树层级**：连接 → 数据库/schema → “表”文件夹 → 表；`.conn-row` / `.db-row` / `.group-row` / `.table-row` 的缩进依次为 10/30/50/90px。表行没有 chevron，所以 `padding-left` 要比“表”文件夹多 40px，否则表名和 schema 名会从同一 x 坐标开始，看起来像没有缩进。

## 提交规范

- 提交信息用**中文**，Conventional Commits 前缀。
- 提交作者统一 `PaienNate <1101839859@qq.com>`。
- 前端改动请把重建后的 `dist` 一起提交。
- 不要提交 `node_modules/`、`.idea/`、`demo.db`。
