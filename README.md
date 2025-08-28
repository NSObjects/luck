````markdown
# 双色球选号工具（Vue + Go + SQLite）

基于 **Vue 3 + Vite** 的前端与 **Gin + SQLite** 的后端，支持：

- 配置化号码生成（与后端 `Config` 类型一一对应）
- 历史开奖导入（Excel），趋势分析（热度 / 热力图）
- 最新一期开奖拉取（可接第三方）
- **go:embed** 将前端打包进后端，产出**单文件二进制**部署

---

## 目录结构

```text
├── backend
│   ├── api/                 # /api/* 各模块路由
│   ├── data/                # 运行期 SQLite（开发环境）
│   ├── draw_latest.go       # /api/draw/latest
│   ├── generator/           # 生成与分析：analysis.go / gen.go
│   ├── main.go              # Gin 入口 + 静态托管（go:embed）
│   ├── store/store.go       # SQLite 封装
│   └── web/dist/            # 前端打包产物（供 go:embed 嵌入）
├── data/                    # 可选：根级 SQLite（忽略进 Git）
├── go.mod
├── go.sum
└── web
    ├── public/              # 静态资源（favicon 等）
    ├── src/                 # Vue 源码（Home.vue / ConfigPanel.vue 等）
    ├── vite.config.js       # outDir 指向 ../backend/web/dist
    ├── package.json
    └── index.html
````

---

## 环境要求

* **Go 1.21+**
* **Node.js 18+**（或 20+）
* **CGO 工具链**（默认使用 `github.com/mattn/go-sqlite3`）

  * Linux: `gcc` / `musl-gcc`
  * macOS: Xcode Command Line Tools
  * （如需纯 Go，可改用 `modernc.org/sqlite`，需同步调整驱动与 DSN）

---

## 一键构建与运行

项目根目录已提供 `Makefile`：**前端打包 → 内嵌后端 → 编译二进制**。

```bash
# 安装依赖（可选）
make tidy

# 构建单文件二进制（内嵌前端）
make build

# 运行
./backend/bin/ssq-app
# 浏览器打开 http://localhost:8080/
```

> **重要**：`web/vite.config.js` 的打包输出为 `../backend/web/dist`（已配置）。
> 构建顺序必须是先 `npm run build`（由 `make build`触发），再 `go build`，go\:embed 才能把静态资源打进去。

### 常用命令

```bash
make build               # 前端打包 + 后端编译（产出 backend/bin/ssq-app）
make run                 # 构建并运行
make clean               # 清理二进制与打包产物
make tidy                # go mod tidy + npm install
make vercel-static       # 仅构建前端到 web/dist（供 Vercel 静态站点）
make release-linux-amd64 # 交叉编译（需对应平台 CGO）
make release-darwin-arm64
```

---

## 前端说明

* 代码目录：`web/`
* 生产部署走**同域** `/api`（代码里有兜底）：

  ```js
  // 例如 Home.vue
  const API = import.meta.env.VITE_API_BASE || `${location.origin}/api`
  ```
* 推荐环境变量：

  * `web/.env.production`

    ```
    VITE_API_BASE=/api
    ```
* `vite.config.js`（关键项）：

  ```js
  export default defineConfig({
    base: '/',
    build: {
      outDir: '../backend/web/dist',
      assetsDir: 'assets',
      emptyOutDir: true,
      target: 'es2017',
    },
  })
  ```

### 主要组件

* `Home.vue`：单页主视图（配置/生成/热度/热力图/汇总）
* `ConfigPanel.vue`：与后端 `Config` 字段对应（UI 不变；在 `Home.vue` 发请求前做枚举/结构体映射）

---

## 后端说明

* **Gin** 提供 API；**go\:embed** 托管前端（`serveSPAEmbedded` 无 `Static/StaticFS` 重定向问题）
* SQLite 数据库存储开奖历史与去重集合（`store/store.go`）

### 默认端口

* `:8080`

---

## API 速览

> 根路径 `/` 为前端应用；以下为同域的后端接口 `/api/*`。

| 方法   | 路径                                 | 说明                                                 |                                              |
| ---- | ---------------------------------- | -------------------------------------------------- | -------------------------------------------- |
| GET  | `/api/config`                      | 获取当前配置（可作为参考/预填）                                   |                                              |
| PUT  | `/api/config`                      | 更新服务端默认配置（校验 `BandTemplates` 和=6）                  |                                              |
| POST | \`/api/history/upload?replace=0    | 1\`                                                | 上传 Excel 历史（Sheet1；第1/2行为表头；第2列日期；第3列 7 行号码） |
| GET  | `/api/history/summary`             | 历史汇总（入库总行数、不重复红球组合数）                               |                                              |
| POST | `/api/generate`                    | 生成号码（请求体：`{ override: boolean, config?: Config }`） |                                              |
| GET  | `/api/analysis/hot?window=50`      | 热/冷分析（近 N 期）                                       |                                              |
| GET  | `/api/analysis/heatmap?window=100` | 热力图数据（近 N 期）                                       |                                              |
| GET  | `/api/draw/latest`                 | 最新一期开奖（支持对齐入库）                                     |                                              |

### 生成接口请求示例

```bash
curl -X POST http://localhost:8080/api/generate \
  -H "Content-Type: application/json" \
  -d '{
    "override": true,
    "config": {
      "Mode": 3,
      "Animal": 11,
      "Birthday": "1991-05-28",
      "GenerateCount": 10,
      "BudgetYuan": 0,
      "RedFilter": [], "BlueFilter": [], "FixedRed": [],
      "FMode": 1, "FixedPerTicket": 2,
      "MaxOverlapRed": 3, "UsePerNumberCap": true,
      "StartBuckets": [{"From":1,"To":10,"Count":3},{"From":11,"To":18,"Count":2},{"From":19,"To":32,"Count":2}],
      "MaxPerAnchor": 1,
      "Bands": {"LowLo":1,"LowHi":11,"MidLo":12,"MidHi":22,"HighLo":23,"HighHi":33},
      "BandTemplates": [[2,2,2],[2,3,1],[3,2,1],[1,2,3],[1,3,2]],
      "TemplateRepeat": 2
    }
  }'
```

> 前端负责把 UI 配置转换为后端 `Config`（枚举数字、字段名）后再发送。

---

## Excel 导入格式

* 工作表名 `Sheet1`（大小写不敏感）
* 第 1/2 行为表头
* 第 2 列：日期（支持 `YYYY-MM-DD`、`YYYY/MM/DD` 等，自动去括号中的星期）
* 第 3 列：7 行号码（6 红 + 1 蓝，每个一行）

---

## 部署

### A. **单文件二进制**

1. 构建

   ```bash
   make build
   ```
2. 运行

   ```bash
   ./backend/bin/ssq-app
   # 访问 http://localhost:8080/
   ```

> 生产建议：
>
> * 反向代理（Nginx/Caddy）+ HTTPS
> * 将 `backend/data`（或 `data/`）挂到持久卷
> * 设置 `GIN_MODE=release`

### B. **Vercel 静态前端 + 独立后端**

* 前端（静态）：

  ```bash
  make vercel-static  # 产出 web/dist
  ```

  Vercel 设置：

  * Root Directory：`web`
  * Output Directory：`dist`
* 后端：部署到长连平台（Railway/Render/Fly.io/VPS 等），并将前端的 `VITE_API_BASE` 指向后端地址。

---

## 常见问题（Troubleshooting）

* **访问 `/` 返回 301**
  使用本项目的 `serveSPAEmbedded`（手动读取 embed 文件并写回），并确保**没有**其它 `Static/StaticFS/StaticFile` 抢路由。
  打包顺序必须：先前端 → 再后端。

* **开发期接口 `ECONNREFUSED`**
  后端未启动或端口不一致。可在 `web/.env.development` 写：

  ```
  VITE_API_BASE=http://localhost:8080/api
  ```

  前端直连后端，不依赖代理。

* **Excel 导入失败**
  检查 Sheet 名是否 *Sheet1*，以及第 3 列是否是 7 行号码（6 红 + 1 蓝，逐行）。

* **CGO/交叉编译失败**
  需要对应平台的 C 编译器。若不便交叉编译，可在目标平台原生构建；或改用 `modernc.org/sqlite`（需同步调整）。

---

## 开发约定

* 前端 **UI 不变**，请求前在 `Home.vue` 将配置**转换为后端 `Config`**（枚举数字、结构体字段对齐）。
* `BandTemplates` 和必须为 6，不足由**中段兜底**。
* 历史去重以**红球组合**为 key（蓝球不参与去重）。

---

## 许可

自定义/内部项目，如需开源可添加 `LICENSE`。

---

## 致谢

* Gin / Vite / Vue
* `mattn/go-sqlite3`
* 以及你提供的生成逻辑与规则配置 🙌

```
```
