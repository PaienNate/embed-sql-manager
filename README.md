# embedSqlClient

把 SQL 控制台做成**可内嵌的 Go 库**：宿主注入已经实现的 `Provider`（`database/sql`、`sqlx`、`sqlex`、GORM 均可），库在自己的 `http.ServeMux` 前缀下挂载一个同源的 SQL 管理页面。

> 核心 `sqlui` 包**只依赖 Go 标准库**（`database/sql`、`net/http`、`embed`），不引入 GORM、任何 SQL 方言实现或数据库驱动。数据库差异全部由宿主的 `Provider` 实现。

## 特性

- **注入式连接**：`Config.Connections` 静态注册，和 `embedRedisClient` 一致；没有“新建连接”入口。
- **左侧树**：连接 → 数据库 → 表（懒加载）。
- **右上 SQL 控制台**：CodeMirror 5 SQL 模式，Ctrl+Enter 执行。
- **右下结果区**：数据 / 结构 / 消息 三个 tab；数据网格最多 1000 行（可配）。默认 SQL 输入占 3/4、结果占 1/4，分隔条可拖动。
- **不做行级编辑**：增删改请在 SQL 控制台写 `INSERT` / `UPDATE` / `DELETE`（需实现 `ExecProvider`），避免各库兼容差异。
- **只读降级**：未实现 `ExecProvider` 的连接，SQL 控制台拒绝非查询语句。
- **IE11**：`@vitejs/plugin-legacy` 产出 ES5 legacy bundle。

## 快速开始

```go
package main

import (
	"log"
	"net/http"

	"embedSqlClient/sqlui"
)

func main() {
	mux := http.NewServeMux()
	err := sqlui.Register(mux, sqlui.Config{
		Prefix:  "/sqlui",
		Version: "myapp-1.0.0",
		Connections: []sqlui.Connection{
			{
				ID:       "local",
				Name:     "Local DB",
				Type:     "SQLite",
				Provider: myProvider, // 你实现的 Provider
				Enabled:  true,
			},
		},
		// Middleware: 你的鉴权中间件（可选，包裹全部路由）
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(http.ListenAndServe(":8080", mux)) // 访问 http://localhost:8080/sqlui/
}
```

## Provider 接口

```go
type Provider interface {
    GetDatabases(ctx context.Context) ([]string, error)
    GetTables(ctx context.Context, database string) ([]string, error)
    GetColumns(ctx context.Context, database, table string) ([]ColumnInfo, error)
    ExecuteSQL(ctx context.Context, sql string) (*sql.Rows, error)
    SelectTable(ctx context.Context, database, table string, limit int) (*sql.Rows, error)
}
```

- `ExecuteSQL`：SQL 控制台执行查询类语句（SELECT/WITH/SHOW/DESCRIBE/EXPLAIN/PRAGMA...）。
- `SelectTable`：表数据预览；引号和 `LIMIT` 由宿主按方言生成。
- `GetDatabases` 返回的“数据库”是第二层命名空间（MySQL 的库、Postgres 的 schema、SQLite 的 `main` 等）。

可选接口：

```go
// 没实现 = SQL 控制台只读（只能 SELECT/WITH/SHOW/...）
type ExecProvider interface {
    ExecSQL(ctx context.Context, sql string) (ExecResult, error)
}
```

## 接入 sys_api_service（参考）

你的 `pkg/database.QueryExecutor` 已经非常接近这个接口，包一层即可：

```go
type provider struct{ q database.QueryExecutor }

func (p provider) GetDatabases(ctx context.Context) ([]string, error) { return p.q.GetDatabases(ctx) }
func (p provider) GetTables(ctx context.Context, db string) ([]string, error) { return p.q.GetTables(ctx, db) }
func (p provider) GetColumns(ctx context.Context, db, table string) ([]sqlui.ColumnInfo, error) {
    cols, err := p.q.GetColumns(ctx, db, table)
    // map database.ColumnInfo -> sqlui.ColumnInfo
}
func (p provider) ExecuteSQL(ctx context.Context, sql string) (*sql.Rows, error) { return p.q.ExecuteSQL(ctx, sql) }
func (p provider) SelectTable(ctx context.Context, db, table string, limit int) (*sql.Rows, error) {
    gdb, err := p.q.GetDB(ctx)
    if err != nil { return nil, err }
    return gdb.Table(table).Limit(limit).Rows()
}
// 可选：ExecSQL 用 gdb.Exec（否则控制台只读）
```

启动时把 `DatabaseManager.GetSourceList()` 转成 `[]sqlui.Connection` 静态注册即可。

## 端点

| 方法 | 路径 | 作用 |
|---|---|---|
| GET | `{prefix}/api/connections` | 连接列表（含 capabilities） |
| GET | `{prefix}/api/sql/{id}/databases` | 数据库/模式列表 |
| GET | `{prefix}/api/sql/{id}/tables?database=` | 表列表 |
| GET | `{prefix}/api/sql/{id}/columns?database=&table=` | 列结构 |
| GET | `{prefix}/api/sql/{id}/table-data?database=&table=&limit=` | 表数据（≤1000 行） |
| POST | `{prefix}/api/sql/{id}/query` | 执行 SQL |

## 构建与测试

```bash
# Go（Go 1.23+）
go build ./...
go test ./...
gofmt -l sqlui/ example/

# 前端（dist 已提交，改前端后需重建）
cd sqlui/web
npm install
npm run subset-icons
npm run build

# 示例（SQLite，写 demo.db）
go run ./example
```

## Docker 运行

仓库根目录提供了 `Dockerfile`（多阶段构建，CGO 编译 SQLite 驱动）和 `docker-compose.yml`：

```bash
docker compose up -d --build
# 打开 http://127.0.0.1:8082/sqlui/
# 或宿主网段：http://172.21.4.31:8082/sqlui/  http://100.78.41.39:8082/sqlui/

docker compose logs -f      # 查看日志
docker compose down         # 停止
docker compose down -v      # 停止并删除数据卷
```

- 容器内监听 `:8081`，宿主映射 `8082`。
- SQLite 数据在命名卷 `sqlui-data`（`/data/demo.db`），重启不丢。
- 可通过环境变量覆盖：`SQLUI_ADDR`（监听地址）、`SQLUI_DB`（数据库文件）。

## 目录

```
embedSqlClient/
  sqlui/           # 库核心：接口、HTTP 端点、*sql.Rows 序列化、embed
    web/           # Vue2 前端（Vite + Element UI + CodeMirror + vxe-table）
  example/         # SQLite Provider 参考实现（database/sql + mattn/go-sqlite3）
```
