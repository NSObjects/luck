package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"luck/backend/generator"
	"luck/backend/store"
	"math"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"resty.dev/v3"
)

/* ===================== 与前端/第三方一致的结构 ===================== */

// 直接复用 store.Draw，避免重复定义
type Draw = store.Draw

type Series struct{ List []Draw }

/* ===================== 基础配置（可通过接口修改） ===================== */

type BandRange struct{ Low, Mid, High [2]int }
type BandTpl struct{ Vals [3]int }

type AppConfig struct {
	Port            int       `json:"port"`
	AllowOrigins    []string  `json:"allow_origins"`
	Count           int       `json:"count"`
	Mode            string    `json:"mode"`
	Animal          string    `json:"animal"`
	Birthday        string    `json:"birthday"`
	RedFilter       []int     `json:"red_filter"`
	BlueFilter      []int     `json:"blue_filter"`
	FixedRed        []int     `json:"fixed_red"`
	FixedMode       string    `json:"fixed_mode"`
	FixedPerTicket  int       `json:"fixed_per_ticket"`
	MaxOverlapRed   int       `json:"max_overlap_red"`
	UsePerNumberCap bool      `json:"use_per_number_cap"`
	Bands           BandRange `json:"bands"`
	BandTemplates   []BandTpl `json:"band_templates"`
	TemplateRepeat  int       `json:"template_repeat"`
	UseAPISource    bool      `json:"use_api_source"`
	APIProvider     string    `json:"api_provider"`
	APIKey          string    `json:"api_key"`
}

var cfg = AppConfig{
	Port:         8080,
	AllowOrigins: []string{"http://localhost:5173", "http://127.0.0.1:5173"},
	Count:        10, Mode: "mixed", Animal: "Dog", Birthday: "1991-05-28",
	RedFilter: []int{}, BlueFilter: []int{}, FixedRed: []int{},
	FixedMode: "rotate", FixedPerTicket: 2, MaxOverlapRed: 3, UsePerNumberCap: true,
	Bands:          BandRange{Low: [2]int{1, 11}, Mid: [2]int{12, 22}, High: [2]int{23, 33}},
	BandTemplates:  []BandTpl{{[3]int{2, 2, 2}}, {[3]int{2, 3, 1}}, {[3]int{3, 2, 1}}, {[3]int{1, 2, 3}}, {[3]int{1, 3, 2}}},
	TemplateRepeat: 2,
	UseAPISource:   false, APIProvider: "jisu", APIKey: "",
}

/* ===================== 服务启动 ===================== */

var st *store.Store

//go:embed web/dist
var embeddedDist embed.FS

func serveSPAEmbedded(r *gin.Engine) {
	// 关闭 Gin 的“自动修正/补斜杠”重定向
	r.RedirectTrailingSlash = false
	r.RedirectFixedPath = false

	// 子 FS 指向 web/dist
	sub, err := fs.Sub(embeddedDist, "web/dist")
	if err != nil {
		panic(err)
	}

	// —— 1) 根与 index.html：直接读文件写回，避免任何目录逻辑 —— //
	r.GET("/", func(c *gin.Context) {
		data, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			c.String(http.StatusNotFound, "index not found")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})
	r.GET("/index.html", func(c *gin.Context) {
		data, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			c.String(http.StatusNotFound, "index not found")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	// —— 2) favicon（如果有就返回，没有就 204） —— //
	r.GET("/favicon.ico", func(c *gin.Context) {
		data, err := fs.ReadFile(sub, "favicon.ico")
		if err != nil {
			c.Status(http.StatusNoContent)
			return
		}
		c.Data(http.StatusOK, "image/x-icon", data)
	})

	// —— 3) 资产：/assets/*filepath —— //
	r.GET("/assets/*filepath", func(c *gin.Context) {
		fp := strings.TrimPrefix(c.Param("filepath"), "/")
		// 防穿越
		clean := path.Clean(fp)
		if clean == "." || strings.Contains(clean, "..") || strings.HasSuffix(clean, "/") {
			c.Status(http.StatusNotFound)
			return
		}
		data, err := fs.ReadFile(sub, path.Join("assets", clean))
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		ct := mime.TypeByExtension(path.Ext(clean))
		if ct == "" {
			ct = "application/octet-stream"
		}
		c.Data(http.StatusOK, ct, data)
	})

	// —— 4) 兜底：非 /api/*、非 /assets/* 的路由全部回退到 index.html —— //
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/assets/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		data, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			c.String(http.StatusNotFound, "index not found")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})
}

func main() {
	var err error
	st, err = store.Open("data")
	if err != nil {
		panic(err)
	}
	defer st.Close()

	r := gin.Default()
	serveSPAEmbedded(r)
	// CORS（开发期放开；同域部署可收紧）
	c := cors.Config{
		AllowOrigins:     cfg.AllowOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "OPTIONS"},
		AllowHeaders:     []string{"*"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}
	// 若 AllowOrigins 为空则允许所有
	if len(c.AllowOrigins) == 0 {
		c = cors.Config{
			AllowAllOrigins: true, AllowMethods: []string{"GET", "POST", "PUT", "OPTIONS"},
			AllowHeaders: []string{"*"}, ExposeHeaders: []string{"Content-Length"},
			AllowCredentials: false, MaxAge: 12 * time.Hour,
		}
	}
	r.Use(cors.New(c))

	registerRoutes(r)

	addr := fmt.Sprintf(":%d", cfg.Port)
	_ = r.Run(addr)
}

/* ===================== 路由注册 ===================== */

func registerRoutes(r *gin.Engine) {
	api := r.Group("/api")

	// 配置
	api.GET("/config", func(ctx *gin.Context) { ctx.JSON(200, cfg) })
	api.PUT("/config", func(ctx *gin.Context) {
		var in AppConfig
		if err := ctx.ShouldBindJSON(&in); err != nil {
			ctx.JSON(400, gin.H{"error": err.Error()})
			return
		}
		for _, t := range in.BandTemplates {
			if t.Vals[0]+t.Vals[1]+t.Vals[2] != 6 {
				ctx.JSON(400, gin.H{"error": "band template must sum to 6"})
				return
			}
		}
		cfg = in
		ctx.JSON(200, gin.H{"ok": true})
	})

	// 历史 Excel 上传（sheet1；跳过前两行表头；第2列日期；第3列为7行号码）
	api.POST("/history/upload", uploadHistoryHandler)
	api.GET("/history/summary", historySummaryHandler)

	// 最新一期（第三方拉取 → 与 DB 对齐 → 返回第三方字段）
	api.GET("/draw/latest", handleLatestDraw)

	// 生成号码（示例：简单随机 + 与历史去重）
	api.POST("/generate", handleGenerate)

	// 分析
	api.GET("/analysis/heatmap", func(ctx *gin.Context) {
		window := atoiDefault(ctx.Query("window"), 100)
		series := seriesRecent(window)
		ctx.JSON(200, BuildHeatmap(series))
	})
	api.GET("/analysis/hot", func(ctx *gin.Context) {
		window := atoiDefault(ctx.Query("window"), 50)
		series := seriesRecent(window)
		ctx.JSON(200, AnalyzeHotCold(series))
	})
	api.GET("/analysis/summary", func(ctx *gin.Context) {
		series := seriesRecent(0)
		ctx.JSON(200, AnalyzeSummary(series))
	})
}

/* ===================== 历史上传 & 汇总 ===================== */

func uploadHistoryHandler(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	replace := c.DefaultQuery("replace", "0") == "1"

	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("hist-%d.xlsx", time.Now().UnixNano()))
	if err := c.SaveUploadedFile(fh, tmp); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer os.Remove(tmp)

	n, e := st.ImportExcel(tmp, replace)
	if e != nil {
		// ErrAlreadyInitialized 时返回 409
		if e.Error() == "already_initialized" {
			c.JSON(409, gin.H{"error": "already_initialized", "msg": "历史已初始化。如需覆盖，请加 ?replace=1"})
			return
		}
		c.JSON(400, gin.H{"error": e.Error()})
		return
	}
	sum, _ := st.HistorySummary()
	c.JSON(200, gin.H{"ok": true, "mode": map[bool]string{true: "replace", false: "init"}[replace], "imported": n, "summary": sum})
}

func historySummaryHandler(c *gin.Context) {
	sum, err := st.HistorySummary()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, sum)
}

/* ===================== 最新一期：第三方拉取 + DB 对齐 ===================== */

func handleLatestDraw(c *gin.Context) {
	ctx := c.Request.Context()

	ext, err := FetchLatestDraw(ctx) // 从第三方拉取
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if ext == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "latest draw not found"})
		return
	}
	if err := validateDraw(*ext); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid draw: " + err.Error()})
		return
	}
	if ext.FetchedAt.IsZero() {
		ext.FetchedAt = time.Now()
	}

	// 与数据库对齐（按 issue 幂等）
	status, prev, err := st.ReconcileIssue(store.Draw{
		Issue:     ext.Issue,
		DrawDate:  ext.DrawDate,
		Reds:      ext.Reds,
		Blue:      ext.Blue,
		Source:    ext.Source,
		FetchedAt: ext.FetchedAt,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "persist failed: " + err.Error()})
		return
	}

	// 默认仅返回第三方字段；若 ?verbose=1 则带上 db_status & prev
	if c.DefaultQuery("verbose", "0") == "1" {
		resp := gin.H{
			"issue":      ext.Issue,
			"draw_date":  trimDate(ext.DrawDate),
			"reds":       ext.Reds,
			"blue":       ext.Blue,
			"source":     ext.Source,
			"fetched_at": ext.FetchedAt,
			"db_status":  status, // inserted | updated | noop
		}
		if prev != nil && status != "noop" {
			resp["prev"] = prev
		}
		c.JSON(http.StatusOK, resp)
		return
	}
	c.JSON(http.StatusOK, ext)
}

// 第三方拉取（你可替换为你的源/密钥）
var FetchLatestDraw = func(ctx context.Context) (*Draw, error) {
	client := resty.New().SetTimeout(8 * time.Second)
	var l luck
	_, err := client.R().SetResult(&l).SetContext(ctx).SetQueryParams(map[string]string{
		"code":       "ssq",
		"app_id":     "rcdrixmfzrmxho3s",
		"app_secret": "dkJQRXNlS0dscW44cFZTbnVlbUVvdz09",
	}).Get("https://www.mxnzp.com/api/lottery/common/latest")
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if l.Code != 1 {
		return nil, fmt.Errorf("invalid response code: %d", l.Code)
	}
	red, blue, err := ParseLineInts(l.Data.OpenCode)
	if err != nil {
		return nil, err
	}

	date := trimDate(l.Data.Time)

	// 防守式升序
	for i := 1; i < len(red); i++ {
		if red[i] < red[i-1] {
			sort.Ints(red)
			break
		}
	}

	return &Draw{
		Issue:     l.Data.Expect,
		DrawDate:  date,
		Reds:      red,
		Blue:      blue,
		Source:    "crawler",
		FetchedAt: time.Now(),
	}, nil
}

type luck struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		OpenCode string `json:"openCode"`
		Code     string `json:"code"`
		Expect   string `json:"expect"`
		Name     string `json:"name"`
		Time     string `json:"time"`
	} `json:"data"`
}

/* ===================== 生成（简单随机示例） ===================== */

type Stats struct {
	RedFreq   map[int]int    `json:"red_freq"`
	BlueFreq  map[int]int    `json:"blue_freq"`
	BandShare map[string]int `json:"band_share"`
	OddEven   map[string]int `json:"odd_even"`
	HighLow   map[string]int `json:"high_low"`
}
type GenerateRequest struct {
	Override bool              `json:"override"`
	Config   *generator.Config `json:"config,omitempty"`
}
type GenerateResponse struct {
	Combos []generator.Combo `json:"combos"`
	Stats  *Stats            `json:"stats,omitempty"`
}

func handleGenerate(ctx *gin.Context) {
	var req GenerateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	//use := cfg
	//if req.Override && req.Config != nil {
	//	use = *req.Config
	//}
	//hist, _, _ := st.HistorySetAndFreq()
	//cfg.
	combos, err := generator.LuckCombo(*req.Config)
	if err != nil {
		return
	}
	ctx.JSON(http.StatusOK, GenerateResponse{Combos: combos, Stats: buildStats(combos, cfg.Bands)})
}

//func generateSimple(n int, hist map[string]struct{}) []Combo {
//	r := rand.New(rand.NewSource(time.Now().UnixNano()))
//	out := make([]Combo, 0, n)
//	for len(out) < n {
//		seen := map[int]struct{}{}
//		reds := make([]int, 0, 6)
//		for len(reds) < 6 {
//			v := 1 + r.Intn(33)
//			if _, ok := seen[v]; ok {
//				continue
//			}
//			seen[v] = struct{}{}
//			reds = append(reds, v)
//		}
//		sort.Ints(reds)
//		key := redKeyStr(reds)
//		if _, dup := hist[key]; dup {
//			continue // 与历史红球重复则跳过
//		}
//		blue := 1 + r.Intn(16)
//		out = append(out, Combo{Reds: reds, Blue: blue})
//	}
//	return out
//}

func buildStats(combos []generator.Combo, bands BandRange) *Stats {
	s := &Stats{
		RedFreq: map[int]int{}, BlueFreq: map[int]int{},
		BandShare: map[string]int{"low": 0, "mid": 0, "high": 0},
		OddEven:   map[string]int{"odd": 0, "even": 0},
		HighLow:   map[string]int{"low": 0, "high": 0},
	}
	midStart := bands.Mid[0]
	for _, c := range combos {
		for _, n := range c.Reds {
			s.RedFreq[n]++
			if n%2 == 0 {
				s.OddEven["even"]++
			} else {
				s.OddEven["odd"]++
			}
			if n < midStart {
				s.HighLow["low"]++
			} else {
				s.HighLow["high"]++
			}
			switch {
			case n >= bands.Low[0] && n <= bands.Low[1]:
				s.BandShare["low"]++
			case n >= bands.Mid[0] && n <= bands.Mid[1]:
				s.BandShare["mid"]++
			default:
				s.BandShare["high"]++
			}
		}
		s.BlueFreq[c.Blue]++
	}
	return s
}

func redKeyStr(red []int) string {
	var b strings.Builder
	for i, v := range red {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(fmt.Sprintf("%02d", v))
	}
	return b.String()
}

/* ===================== 分析：热/冷、热力图、汇总 ===================== */

type HotCold struct {
	RedFreq    map[int]int     `json:"redFreq"`
	BlueFreq   map[int]int     `json:"blueFreq"`
	TopHotRed  [][2]int        `json:"topHotRed"`
	TopColdRed [][2]int        `json:"topColdRed"`
	AvgGapRed  map[int]float64 `json:"avgGapRed"`
	MaxGapRed  map[int]int     `json:"maxGapRed"`
	MA33       []float64       `json:"MA33"`
}

func AnalyzeHotCold(s Series) HotCold {
	h := HotCold{
		RedFreq:   make(map[int]int, 33),
		BlueFreq:  make(map[int]int, 16),
		AvgGapRed: make(map[int]float64, 33),
		MaxGapRed: make(map[int]int, 33),
	}
	// 频次
	for _, d := range s.List {
		for _, r := range d.Reds {
			h.RedFreq[r]++
		}
		if d.Blue >= 1 && d.Blue <= 16 {
			h.BlueFreq[d.Blue]++
		}
	}
	// 间隔
	lastIdx := make(map[int]int, 33)
	totalGap := make(map[int]int, 33)
	countGap := make(map[int]int, 33)
	for i, d := range s.List {
		seen := map[int]struct{}{}
		for _, r := range d.Reds {
			seen[r] = struct{}{}
		}
		for n := 1; n <= 33; n++ {
			if _, ok := seen[n]; ok {
				if j, ok2 := lastIdx[n]; ok2 {
					gap := i - j
					totalGap[n] += gap
					countGap[n]++
					if gap > h.MaxGapRed[n] {
						h.MaxGapRed[n] = gap
					}
				}
				lastIdx[n] = i
			}
		}
	}
	for n := 1; n <= 33; n++ {
		if countGap[n] > 0 {
			h.AvgGapRed[n] = float64(totalGap[n]) / float64(countGap[n])
		} else {
			h.AvgGapRed[n] = math.NaN()
		}
	}
	// 热/冷榜
	type pair struct{ n, f int }
	arr := make([]pair, 0, 33)
	for n := 1; n <= 33; n++ {
		arr = append(arr, pair{n, h.RedFreq[n]})
	}
	sort.Slice(arr, func(i, j int) bool {
		if arr[i].f == arr[j].f {
			return arr[i].n < arr[j].n
		}
		return arr[i].f > arr[j].f
	})
	for i := 0; i < min(6, len(arr)); i++ {
		h.TopHotRed = append(h.TopHotRed, [2]int{arr[i].n, arr[i].f})
	}
	sort.Slice(arr, func(i, j int) bool {
		if arr[i].f == arr[j].f {
			return arr[i].n < arr[j].n
		}
		return arr[i].f < arr[j].f
	})
	for i := 0; i < min(6, len(arr)); i++ {
		h.TopColdRed = append(h.TopColdRed, [2]int{arr[i].n, arr[i].f})
	}
	// 移动平均（示例）
	sum := 0.0
	window := 5
	for i := 0; i < 33; i++ {
		f := float64(h.RedFreq[i+1])
		sum += f
		if i >= window {
			sum -= float64(h.RedFreq[i+1-window])
		}
		if i >= window-1 {
			h.MA33 = append(h.MA33, sum/float64(window))
		}
	}
	return h
}

type Heatmap struct {
	RedMatrix  [][]int `json:"redMatrix"`
	BlueVector []int   `json:"blueVector"`
}

func BuildHeatmap(s Series) Heatmap {
	N := len(s.List)
	m := Heatmap{
		RedMatrix:  make([][]int, 33),
		BlueVector: make([]int, N),
	}
	for i := 0; i < 33; i++ {
		m.RedMatrix[i] = make([]int, N)
	}
	for j, d := range s.List {
		set := map[int]struct{}{}
		for _, r := range d.Reds {
			set[r] = struct{}{}
		}
		for n := 1; n <= 33; n++ {
			if _, ok := set[n]; ok {
				m.RedMatrix[n-1][j] = 1
			}
		}
		m.BlueVector[j] = d.Blue
	}
	return m
}

type Summary struct {
	Odd, Even      int         `json:"odd"`
	Low, High      int         `json:"low"`
	Area           [3]int      `json:"area"`
	SumMin, SumMax float64     `json:"sumMin"`
	SumAvg         float64     `json:"sumAvg"`
	ConsecLenDist  map[int]int `json:"consecLenDist"`
	ChiSquare      float64     `json:"chiSquare"`
	Entropy        float64     `json:"entropy"`
}

func AnalyzeSummary(s Series) Summary {
	var sumAll float64
	var sumCnt float64
	out := Summary{ConsecLenDist: map[int]int{}}
	for _, d := range s.List {
		sum := 0
		for _, r := range d.Reds {
			sum += r
			if r%2 == 0 {
				out.Even++
			} else {
				out.Odd++
			}
			if r <= 16 {
				out.Low++
			} else {
				out.High++
			}
			switch {
			case r >= 1 && r <= 11:
				out.Area[0]++
			case r >= 12 && r <= 22:
				out.Area[1]++
			default:
				out.Area[2]++
			}
		}
		sumAll += float64(sum)
		sumCnt++
		// 连号
		clen := 1
		for i := 1; i < len(d.Reds); i++ {
			if d.Reds[i] == d.Reds[i-1]+1 {
				clen++
			} else {
				if clen >= 2 {
					out.ConsecLenDist[clen]++
				}
				clen = 1
			}
		}
		if clen >= 2 {
			out.ConsecLenDist[clen]++
		}
	}
	if sumCnt > 0 {
		out.SumAvg = sumAll / sumCnt
	}
	// 卡方 & 熵
	total := float64(len(s.List) * 6)
	exp := total / 33.0
	ch := 0.0
	ent := 0.0
	for n := 1; n <= 33; n++ {
		cnt := 0
		for _, d := range s.List {
			for _, r := range d.Reds {
				if r == n {
					cnt++
				}
			}
		}
		p := float64(cnt) / total
		diff := float64(cnt) - exp
		ch += diff * diff / exp
		if p > 0 {
			ent += -p * math.Log2(p)
		}
	}
	out.ChiSquare = ch
	out.Entropy = ent
	// 和值范围（近50期）
	window := min(50, len(s.List))
	if window > 0 {
		minv, maxv := math.MaxInt, math.MinInt
		for _, d := range s.List[len(s.List)-window:] {
			su := 0
			for _, r := range d.Reds {
				su += r
			}
			if su < minv {
				minv = su
			}
			if su > maxv {
				maxv = su
			}
		}
		out.SumMin, out.SumMax = float64(minv), float64(maxv)
	}
	return out
}

/* ===================== 工具函数 ===================== */

func seriesRecent(window int) Series {
	list, err := st.ListRecentDraws(window) // window<=0 → 全量；否则近 N 期（升序）
	if err != nil {
		return Series{List: []Draw{}}
	}
	return Series{List: list}
}

func atoiDefault(s string, def int) int {
	var x int
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &x); err == nil {
		return x
	}
	return def
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func trimDate(s string) string {
	ss := strings.TrimSpace(s)
	if len(ss) >= 10 {
		return ss[:10]
	}
	return ss
}

func ParseLineInts(s string) ([]int, int, error) {
	s = strings.TrimSpace(s)
	left, right, ok := strings.Cut(s, "+")
	if !ok {
		return nil, 0, fmt.Errorf("missing '+' part")
	}
	// 解析红球
	var red []int
	for i, part := range strings.Split(left, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return nil, 0, fmt.Errorf("invalid number at index %d: %v", i, err)
		}
		if n < 1 || n > 33 {
			return nil, 0, fmt.Errorf("red out of range: %d", n)
		}
		red = append(red, n)
	}
	// 解析蓝球
	blue, err := strconv.Atoi(strings.TrimSpace(right))
	if err != nil {
		return nil, 0, fmt.Errorf("invalid plus number: %v", err)
	}
	if blue < 1 || blue > 16 {
		return nil, 0, fmt.Errorf("blue out of range: %d", blue)
	}
	return red, blue, nil
}

func validateDraw(d Draw) error {
	if len(d.Reds) != 6 {
		return fmt.Errorf("reds must be 6 numbers")
	}
	last := 0
	for _, v := range d.Reds {
		if v < 1 || v > 33 {
			return fmt.Errorf("red out of range: %d", v)
		}
		if v <= last {
			return fmt.Errorf("reds must be strictly increasing")
		}
		last = v
	}
	if d.Blue < 1 || d.Blue > 16 {
		return fmt.Errorf("blue out of range: %d", d.Blue)
	}
	return nil
}
