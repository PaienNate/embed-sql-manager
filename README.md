# embedSqlClient

把 SQL 控制台做成**可内嵌的 Go 库**：宿主注入已经实现的 `Provider`（`database/sql`、`sqlx`、`sqlex`、GORM 均可），库在自己的 `http.ServeMux` 前缀下挂载一个同源的 SQL 管理页面。

> 核心 `sqlui` 包**只依赖 Go 标准库**（`database/sql`、`net/http`、`embed`），不引入 GORM、任何 SQL 方言实现或数据库驱动。数据库差异全部由宿主的 `Provider` 实现。

## 特性

- **注入式连接**：`Config.Connections` 静态注册，和 `embedRedisClient` 一致；没有“新建连接”入口。
- **左侧树**：连接 → 数据库/schema → 表文件夹 → 表（懒加载，逐级缩进）；选中数据源会自动展开第一个库/schema 并加载表名补全。
- **多查询页签**：Navicat 风格，新建/关闭/切换，每个页签独立 SQL；新建页签继承当前数据源，SQL 持久化到 localStorage。
- **右上 SQL 控制台**：CodeMirror 5 SQL 模式，Ctrl+Enter 执行，Ctrl+Space 补全（关键字/函数/表名/当前表列名）。
- **右下结果区**：数据 / 结构 / 消息 三个 tab；数据网格最多 1000 行（可配）。默认 SQL 输入占 3/4、结果占 1/4，分隔条可拖动。
- **导入导出**：SQL 文件导入为新页签、导出当前/全部查询；结果集导出 CSV/JSON。不提供“数据导入到表”，写库请用 SQL 控制台。
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

## PostgreSQL 示例（datacenterDB）

`example/` 除 SQLite 外，还注册了一个 `datacenterDB`（PostgreSQL）连接。它的连接配置**不写死在代码里**，
而从环境变量读取（`POSTGRES_HOST` / `POSTGRES_PORT` / `POSTGRES_USER` / `POSTGRES_PASSWORD` /
`POSTGRES_DBNAME` / `POSTGRES_CONFIG`），因此不会把真实主机、库名或密码提交进仓库：

```text
# 通过环境变量注入连接设置，例如：
#   POSTGRES_HOST=<your-pg-host> POSTGRES_DBNAME=<your-db> POSTGRES_PASSWORD=<your-password>
# 未设置 POSTGRES_HOST 时，该连接会被跳过（demo 只注册 SQLite）。
host=$POSTGRES_HOST port=$POSTGRES_PORT user=$POSTGRES_USER dbname=$POSTGRES_DBNAME $POSTGRES_CONFIG
```

- Provider 实现在 `example/postgres.go`（`database/sql` + `github.com/lib/pq`，只有 example 依赖它）。
- 元数据查询（schema / 表 / 列 / 主键 / 注释）参考了 `sys_api_service` 的 `PostgresQueryExecutor`。
- `GetDatabases` 返回 **schema**；`current_schema()`（即 `search_path` 的第一个条目）排在最前，树会自动展开它。
- SQL 控制台按 DSN 的 `search_path` 执行；表数据预览会用选中的 schema 限定表名（`"<schema>"."<table>"`）。
- `strictCreateMode` 是 GORM 的迁移保护，不影响这里的原始 SQL；控制台能执行什么由数据库账号权限决定。
- 数据库暂时不可达时 example 仍会启动，连接保留，打开树时返回错误；其他连接不受影响。

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

# 示例（SQLite 写 demo.db + datacenterDB/PostgreSQL）
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
- `datacenterDB`（PostgreSQL）示例连接是否启用由容器里的 `POSTGRES_*` 环境变量决定；容器需要能解析并访问对应的 Postgres 主机。如果容器 DNS 不通，可在 compose 里加 `extra_hosts` 或改用宿主网络。
- 可通过环境变量覆盖：`SQLUI_ADDR`（监听地址）、`SQLUI_DB`（数据库文件）。

## 目录

```
embedSqlClient/
  sqlui/           # 库核心：接口、HTTP 端点、*sql.Rows 序列化、embed
    web/           # Vue2 前端（Vite + Element UI + CodeMirror + vxe-table）
  example/         # SQLite + PostgreSQL Provider 参考实现（database/sql）
```
