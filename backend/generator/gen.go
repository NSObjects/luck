package generator

import (
	"fmt"
	"log"
	"luck/backend/store"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

const (
	excelPath          = "xxd.xlsx"
	sheetName          = "Sheet1"
	pricePerTicketYuan = 2
)

/* =============================== 默认配置 & 校验 =============================== */

func DefaultConfig() Config {
	return Config{
		Mode:            ModeMixed,
		Animal:          Goat,
		Birthday:        "1991-05-28",
		GenerateCount:   10,
		BudgetYuan:      0,
		RedFilter:       []int{},
		BlueFilter:      []int{},
		FixedRed:        []int{},
		FMode:           FixedRotate,
		FixedPerTicket:  1,
		MaxOverlapRed:   3,
		UsePerNumberCap: true,

		StartBuckets: []StartBucket{
			{From: 1, To: 10, Count: 3},
			{From: 11, To: 18, Count: 2},
			{From: 19, To: 32, Count: 2},
		},
		MaxPerAnchor: 1,

		// 分段：低/中/高
		Bands: BandRange{LowLo: 1, LowHi: 11, MidLo: 12, MidHi: 22, HighLo: 23, HighHi: 33},

		// 分段模板（和为 6），轮转使用
		BandTemplates: [][3]int{
			{2, 2, 2},
			{2, 3, 1},
			{3, 2, 1},
			{1, 2, 3},
			{1, 3, 2},
		},
		TemplateRepeat: 2,
	}
}

func enforceBudget(cfg *Config) {
	if cfg.BudgetYuan > 0 {
		maxByBudget := cfg.BudgetYuan / pricePerTicketYuan
		if maxByBudget < cfg.GenerateCount {
			cfg.GenerateCount = maxByBudget
		}
	}
	if cfg.GenerateCount <= 0 {
		log.Fatalf("生成注数必须 > 0")
	}
}

/* =============================== 基础枚举与配置 =============================== */

type Mode int

const (
	ModeRandom Mode = iota
	ModeZodiac      // 生肖模式
	ModeBirthday
	ModeMixed
)

// 12 生肖
type ChineseZodiac int

const (
	Rat ChineseZodiac = iota + 1
	Ox
	Tiger
	Rabbit
	Dragon
	Snake
	Horse
	Goat
	Monkey
	Rooster
	Dog
	Pig
)

type FixedMode int

const (
	FixedAlways FixedMode = iota
	FixedRotate
)

type StartBucket struct {
	From  int // 锚点最小值（含）
	To    int // 锚点最大值（含，<=0 表示不限；内部会与 28 取 min）
	Count int // 该区间需要多少注
}

type BandRange struct {
	LowLo, LowHi   int // 默认 1..11
	MidLo, MidHi   int // 默认 12..22
	HighLo, HighHi int // 默认 23..33
}

type Config struct {
	// 选号策略
	Mode     Mode
	Animal   ChineseZodiac // 生肖
	Birthday string        // YYYY-MM-DD

	// 注数/预算
	GenerateCount int
	BudgetYuan    int

	// 过滤/固定
	RedFilter  []int
	BlueFilter []int
	FixedRed   []int

	// 幸运号策略
	FMode          FixedMode
	FixedPerTicket int // Rotate 下每注放入的幸运号数量（1~2 建议）

	// 覆盖控制
	MaxOverlapRed   int  // 任意两注红球最大重叠数（2~3 建议）
	UsePerNumberCap bool // 单号出现次数上限（自动按注数计算）

	// 锚点配置
	StartBuckets []StartBucket
	MaxPerAnchor int // 同一锚点最多出现几次（建议 1）

	// 分段模板（控制整体分布）
	Bands          BandRange
	BandTemplates  [][3]int // 每项为 {Low, Mid, High}，三者和必须为 6
	TemplateRepeat int      // 每个模板连续使用多少注（默认 2）
}

/* =============================== 生成器封装 =============================== */

type Generator struct {
	cfg           Config
	r             *rand.Rand
	nextRow       int
	redHistory    map[string]struct{} // 历史红球组合
	histFreq      [34]int             // 历史频次（1..33）
	currFreq      [34]int             // 本次频次（1..33）
	generatedReds [][6]int            // 本轮已生成红球组合
	capPerNumber  int                 // 单号 cap（若启用）
	blueSeq       []int               // 蓝球序列（确定性）
	anchorPlan    []int               // 锚点序列（确定性）
	luckyAll      []int               // 幸运号全集（确定性）
}

/* =============================== main =============================== */

func LuckCombo(cfg Config) ([]Combo, error) {
	enforceBudget(&cfg)

	g := mustNewGenerator(cfg)

	g.planBlueSequence()
	g.planAnchorSequence()
	g.prepareLuckyList()

	return g.generateAndWriteAll()

}

/* =============================== Generator 构造 & 规划 =============================== */

func mustNewGenerator(cfg Config) *Generator {
	//f, err := excelize.OpenFile(excelPath)
	//if err != nil {
	//	log.Fatalf("打开 Excel 失败: %v", err)
	//}
	//redHistory, histFreq, nextRow := readHistoryAndNextRowWithFreq(f)

	// 计算单号 cap
	capPer := 1 << 30
	if cfg.UsePerNumberCap {
		totalSlots := 6 * cfg.GenerateCount
		capPer = (totalSlots + 33 - 1) / 33
		if capPer < 2 {
			capPer = 2
		}
	}
	st, err := store.Open("data")
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			panic(err)
		}
	}()
	redHistory, histFreq, err := st.HistorySetAndFreq()
	if err != nil {
		panic(err)
	}
	return &Generator{
		cfg: cfg,
		r:   rand.New(rand.NewSource(time.Now().UnixNano())),
		//nextRow:      nextRow,
		redHistory:   redHistory,
		histFreq:     histFreq,
		capPerNumber: capPer,
	}
}

func (g *Generator) planBlueSequence() {
	avail := buildAvailableBlues(g.cfg.BlueFilter)
	if len(avail) == 0 {
		log.Fatalf("蓝球可用列表为空（过滤过严？）")
	}
	n := g.cfg.GenerateCount
	L := len(avail)
	base := append([]int{}, avail...)

	// 生日优先；否则按生肖
	if y, m, d, ok := parseBirthday(g.cfg.Birthday); ok {
		seed := stableSeedFromBirthday(y, m, d)
		r := rand.New(rand.NewSource(int64(seed)))
		for i := L - 1; i > 0; i-- {
			j := r.Intn(i + 1)
			base[i], base[j] = base[j], base[i]
		}
		offset := seed % L
		g.blueSeq = roundRobin(base, n, offset)
		return
	}
	if g.cfg.Animal >= Rat && g.cfg.Animal <= Pig {
		offset := int(g.cfg.Animal-1) % L
		g.blueSeq = roundRobin(base, n, offset)
		return
	}
	g.blueSeq = roundRobin(base, n, 0)
}

func (g *Generator) planAnchorSequence() {
	g.anchorPlan = buildStartAnchorPlan(g.cfg.GenerateCount, g.cfg.StartBuckets, g.cfg)
}

func (g *Generator) prepareLuckyList() {
	g.luckyAll = buildLuckyList(g.cfg)
}

/* =============================== 总生成流程 =============================== */

func (g *Generator) generateAndWriteAll() ([]Combo, error) {
	redOnce := make(map[string]struct{}, g.cfg.GenerateCount)
	var cbx []Combo
	for i := 0; i < g.cfg.GenerateCount; i++ {
		// 1) 该注的锚点 & 最小值
		anchor := 1
		if i < len(g.anchorPlan) {
			anchor = max(1, g.anchorPlan[i])
		}
		minStart := anchor

		// 2) 该注的幸运号（轮转/固定），并强制包含锚点
		lucky := pickLuckyForTicket(g.cfg, g.luckyAll, i)
		if !contains(lucky, anchor) {
			lucky = append([]int{anchor}, lucky...)
		}

		// 3) 本注分段配比（L/M/H）
		lNeed, mNeed, hNeed := pickBandNeed(g.cfg, i, lucky, minStart)

		// 4) 先按模板严格生成 → 再退让 → 最后兜底
		red, ok := genRedsWithBandTemplate(
			g.cfg, lucky, &g.histFreq, &g.currFreq, g.capPerNumber, g.generatedReds, g.r, minStart, lNeed, mNeed, hNeed,
		)
		if !ok {
			red, ok = tryGenerateWithBandTemplate(
				g.cfg, lucky, &g.histFreq, &g.currFreq, g.capPerNumber, g.generatedReds, g.r, minStart, lNeed, mNeed, hNeed,
			)
		}
		if !ok {
			red, ok = bruteForceUniqueIgnoringConstraints(&g.histFreq, &g.currFreq, g.redHistory, g.r, g.cfg.RedFilter, minStart)
			if !ok {
				return nil, fmt.Errorf("兜底也失败：请降低约束/减少过滤/减少注数")
			}
		}

		sort.Ints(red)
		key := redKeyStr(red)
		if _, dup := g.redHistory[key]; dup {
			i--
			continue
		}
		if _, dup := redOnce[key]; dup {
			i--
			continue
		}

		// 5) 写入 Excel
		blue := g.blueSeq[i]
		//if err := g.writeRow(i, red, blue); err != nil {
		//	return nil, err
		//}

		// 6) 记录状态
		redOnce[key] = struct{}{}
		g.redHistory[key] = struct{}{}
		for _, v := range red {
			g.currFreq[v]++
		}
		g.generatedReds = append(g.generatedReds, toFixedArray6(red))
		cbx = append(cbx, Combo{
			Reds: red,
			Blue: blue,
		})
		fmt.Printf(" %v,  %02d\n", red, blue)
	}
	return cbx, nil
}

type Combo struct {
	Reds []int `json:"reds"`
	Blue int   `json:"blue"`
}

func (g *Generator) generateLuckBall() ([]string, error) {
	redOnce := make(map[string]struct{}, g.cfg.GenerateCount)
	var luckBall []string
	for i := 0; i < g.cfg.GenerateCount; i++ {
		// 1) 该注的锚点 & 最小值
		anchor := 1
		if i < len(g.anchorPlan) {
			anchor = max(1, g.anchorPlan[i])
		}
		minStart := anchor

		// 2) 该注的幸运号（轮转/固定），并强制包含锚点
		lucky := pickLuckyForTicket(g.cfg, g.luckyAll, i)
		if !contains(lucky, anchor) {
			lucky = append([]int{anchor}, lucky...)
		}

		// 3) 本注分段配比（L/M/H）
		lNeed, mNeed, hNeed := pickBandNeed(g.cfg, i, lucky, minStart)

		// 4) 先按模板严格生成 → 再退让 → 最后兜底
		red, ok := genRedsWithBandTemplate(
			g.cfg, lucky, &g.histFreq, &g.currFreq, g.capPerNumber, g.generatedReds, g.r, minStart, lNeed, mNeed, hNeed,
		)
		if !ok {
			red, ok = tryGenerateWithBandTemplate(
				g.cfg, lucky, &g.histFreq, &g.currFreq, g.capPerNumber, g.generatedReds, g.r, minStart, lNeed, mNeed, hNeed,
			)
		}
		if !ok {
			red, ok = bruteForceUniqueIgnoringConstraints(&g.histFreq, &g.currFreq, g.redHistory, g.r, g.cfg.RedFilter, minStart)
			if !ok {
				return nil, fmt.Errorf("兜底也失败：请降低约束/减少过滤/减少注数")
			}
		}

		sort.Ints(red)
		key := redKeyStr(red)
		if _, dup := g.redHistory[key]; dup {
			i--
			continue
		}
		if _, dup := redOnce[key]; dup {
			i--
			continue
		}

		blue := g.blueSeq[i]
		// 6) 记录状态
		redOnce[key] = struct{}{}
		g.redHistory[key] = struct{}{}
		for _, v := range red {
			g.currFreq[v]++
		}
		g.generatedReds = append(g.generatedReds, toFixedArray6(red))
		luckBall = append(luckBall, fmt.Sprintf("%v+%02d", red, blue))

	}
	return luckBall, nil
}

//func (g *Generator) writeRow(idx int, red []int, blue int) error {
//	row := g.nextRow + idx
//	dateCell := "B" + strconv.Itoa(row)
//	numCell := "C" + strconv.Itoa(row)
//
//	if err := g.file.SetCellValue(sheetName, dateCell, time.Now().Format("2006-01-02")); err != nil {
//		return fmt.Errorf("写入日期失败: %w", err)
//	}
//	lines := make([]string, 0, 7)
//	for _, v := range red {
//		lines = append(lines, fmt.Sprintf("%02d", v))
//	}
//	lines = append(lines, fmt.Sprintf("%02d", blue))
//	if err := g.file.SetCellValue(sheetName, numCell, strings.Join(lines, "\n")); err != nil {
//		return fmt.Errorf("写入号码失败: %w", err)
//	}
//	return nil
//}

/* =============================== 规划：锚点区间 =============================== */

func buildStartAnchorPlan(n int, buckets []StartBucket, cfg Config) []int {
	if n <= 0 {
		return nil
	}
	// 稳定随机：生日 > 生肖 > 当前时间
	var seed int64
	if y, m, d, ok := parseBirthday(cfg.Birthday); ok {
		seed = int64(stableSeedFromBirthday(y, m, d))
	} else if cfg.Animal >= Rat && cfg.Animal <= Pig {
		seed = int64(20011 + int(cfg.Animal)*137)
	} else {
		seed = time.Now().UnixNano()
	}
	r := rand.New(rand.NewSource(seed))

	type seq struct {
		items []int
		idx   int
	}
	var sequences []*seq

	clamp := func(x, lo, hi int) int {
		if x < lo {
			return lo
		}
		if x > hi {
			return hi
		}
		return x
	}

	for _, b := range buckets {
		if b.Count <= 0 {
			continue
		}
		lo := b.From
		if lo < 1 {
			lo = 1
		}
		hi := b.To
		if hi <= 0 {
			hi = 28
		}
		hi = clamp(hi, lo, 28) // 28 保证还能取满 6 个数
		width := hi - lo + 1
		if width <= 0 {
			continue
		}
		step := (width + b.Count - 1) / b.Count
		if step < 1 {
			step = 1
		}
		offset := r.Intn(step)
		items := make([]int, 0, b.Count)
		cur := lo + offset
		for len(items) < b.Count {
			if cur > hi {
				cur = lo + (cur - hi - 1)
			}
			items = append(items, cur)
			cur += step
		}
		r.Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })
		sequences = append(sequences, &seq{items: items})
	}

	// 交错合并
	plan := make([]int, 0, n)
	for len(plan) < n && len(sequences) > 0 {
		allEmpty := true
		for _, s := range sequences {
			if s.idx < len(s.items) {
				plan = append(plan, s.items[s.idx])
				s.idx++
				allEmpty = false
				if len(plan) >= n {
					break
				}
			}
		}
		if allEmpty {
			break
		}
	}

	// 限频/补齐
	maxPer := cfg.MaxPerAnchor
	if maxPer <= 0 {
		maxPer = 1
	}
	used := map[int]int{}
	final := make([]int, 0, n)
	for _, a := range plan {
		if used[a] < maxPer {
			final = append(final, a)
			used[a]++
			if len(final) >= n {
				break
			}
		}
	}
	for len(final) < n {
		a := 1 + r.Intn(28)
		if used[a] < maxPer {
			final = append(final, a)
			used[a]++
		}
	}
	return final
}

/* =============================== 分段模板 =============================== */

func pickBandNeed(cfg Config, idx int, lucky []int, minStart int) (l, m, h int) {
	// 轮转模板
	tpls := cfg.BandTemplates
	if len(tpls) == 0 {
		return 2, 2, 2
	}
	rep := cfg.TemplateRepeat
	if rep <= 0 {
		rep = 2
	}
	t := (idx / rep) % len(tpls)
	l, m, h = tpls[t][0], tpls[t][1], tpls[t][2]

	// lucky 中（>=minStart）的号码先占位
	for _, n := range lucky {
		if n < minStart || n < 1 || n > 33 {
			continue
		}
		switch bandOf(cfg.Bands, n) {
		case 0:
			if l > 0 {
				l--
			}
		case 1:
			if m > 0 {
				m--
			}
		case 2:
			if h > 0 {
				h--
			}
		}
	}
	// 矫正
	if l < 0 {
		l = 0
	}
	if m < 0 {
		m = 0
	}
	if h < 0 {
		h = 0
	}
	sum := l + m + h
	if sum > 6 {
		ex := sum - 6
		for ex > 0 && h > 0 {
			h--
			ex--
		}
		for ex > 0 && m > 0 {
			m--
			ex--
		}
		for ex > 0 && l > 0 {
			l--
			ex--
		}
	} else if sum < 6 {
		m += 6 - sum // 中段兜底
	}
	return
}

func bandOf(b BandRange, n int) int {
	if n >= b.LowLo && n <= b.LowHi {
		return 0
	}
	if n >= b.MidLo && n <= b.MidHi {
		return 1
	}
	return 2
}

/* =============================== 红球生成（模板 → 退让 → 兜底） =============================== */

func genRedsWithBandTemplate(
	cfg Config,
	lucky []int,
	histFreq *[34]int,
	currFreq *[34]int,
	capPerNumber int,
	existing [][6]int,
	r *rand.Rand,
	minStart int,
	lNeed, mNeed, hNeed int,
) ([]int, bool) {

	// 1) 固定 lucky（过滤、cap、>=minStart）
	block := toSet(cfg.RedFilter)
	fixed := make([]int, 0, 6)
	seen := map[int]struct{}{}

	tryPushFixed := func(n int) {
		if n < minStart || n < 1 || n > 33 {
			return
		}
		if _, bad := block[n]; bad {
			return
		}
		if histFreq[n]+currFreq[n] >= capPerNumber {
			return
		}
		if _, ok := seen[n]; ok {
			return
		}
		seen[n] = struct{}{}
		if len(fixed) < 6 {
			fixed = append(fixed, n)
		}
	}
	for _, n := range lucky {
		tryPushFixed(n)
	}
	sort.Ints(fixed)

	// 2) 候选（>=minStart、未过滤、未超 cap），频次升序
	cands := make([]int, 0, 33)
	for n := max(1, minStart); n <= 33; n++ {
		if _, bad := block[n]; !bad && (histFreq[n]+currFreq[n] < capPerNumber) {
			cands = append(cands, n)
		}
	}
	if len(cands)+len(fixed) < 6 {
		// 放宽：允许 lucky 超 cap
		for _, n := range lucky {
			if n < minStart || n < 1 || n > 33 {
				continue
			}
			if _, bad := block[n]; bad {
				continue
			}
			if _, ok := seen[n]; ok {
				continue
			}
			seen[n] = struct{}{}
			fixed = append(fixed, n)
			if len(fixed) >= 6 {
				break
			}
		}
	}
	if len(cands)+len(fixed) < 6 {
		return nil, false
	}

	sort.Slice(cands, func(i, j int) bool {
		fi := histFreq[cands[i]] + currFreq[cands[i]]
		fj := histFreq[cands[j]] + currFreq[cands[j]]
		if fi == fj {
			return cands[i] < cands[j]
		}
		return fi < fj
	})

	// 3) 先满足分段需求（尽可能）
	chosen := append([]int{}, fixed...)
	useFromBand := func(band int) (int, bool) {
		for _, n := range cands {
			if contains(chosen, n) {
				continue
			}
			if bandOf(cfg.Bands, n) != band {
				continue
			}
			if cfg.MaxOverlapRed > 0 && !respectOverlap(append(chosen, n), existing, cfg.MaxOverlapRed) {
				continue
			}
			return n, true
		}
		return -1, false
	}

	need := 6 - len(chosen)
	pickBand := func(cnt *int, band int) bool {
		if *cnt <= 0 {
			return false
		}
		n, ok := useFromBand(band)
		if !ok {
			return false
		}
		chosen = append(chosen, n)
		currFreq[n]++
		*cnt--
		need--
		return true
	}

	for need > 0 && (lNeed > 0 || mNeed > 0 || hNeed > 0) {
		if pickBand(&lNeed, 0) || pickBand(&mNeed, 1) || pickBand(&hNeed, 2) {
			continue
		}
		break // 某段取不到 → 跳柔性
	}

	// 4) 柔性补齐：不限制段，仍遵守重叠约束与冷号优先
	for need > 0 {
		picked := -1
		for _, n := range cands {
			if contains(chosen, n) {
				continue
			}
			if cfg.MaxOverlapRed > 0 && !respectOverlap(append(chosen, n), existing, cfg.MaxOverlapRed) {
				continue
			}
			picked = n
			break
		}
		if picked == -1 {
			return nil, false
		}
		chosen = append(chosen, picked)
		currFreq[picked]++
		need--
	}
	return chosen, true
}

func tryGenerateWithBandTemplate(
	baseCfg Config,
	lucky []int,
	histFreq *[34]int,
	currFreq *[34]int,
	baseCap int,
	existing [][6]int,
	r *rand.Rand,
	minStart int,
	lNeed, mNeed, hNeed int,
) ([]int, bool) {

	luckyPlans := [][]int{lucky}
	if len(lucky) > 1 {
		luckyPlans = append(luckyPlans, lucky[:1]) // 降低幸运号强度
	}
	luckyPlans = append(luckyPlans, nil) // 不用幸运号

	for addOv := 0; addOv <= 3; addOv++ {
		ov := min(baseCfg.MaxOverlapRed+addOv, 6)
		for addCap := 0; addCap <= 3; addCap++ {
			capNow := baseCap + addCap
			for idx, plan := range luckyPlans {
				fixedPer := baseCfg.FixedPerTicket
				if idx == 1 {
					fixedPer = min(1, fixedPer)
				}
				if idx == 2 {
					fixedPer = 0
				}
				cfg := baseCfg
				cfg.MaxOverlapRed = ov
				cfg.FixedPerTicket = fixedPer
				cfg.UsePerNumberCap = true
				if red, ok := genRedsWithBandTemplate(cfg, plan, histFreq, currFreq, capNow, existing, r, minStart, lNeed, mNeed, hNeed); ok {
					return red, true
				}
			}
		}
	}
	return nil, false
}

/* =============================== 兜底 & 重叠 & 历史 =============================== */

func bruteForceUniqueIgnoringConstraints(
	histFreq *[34]int,
	currFreq *[34]int,
	redHistory map[string]struct{},
	r *rand.Rand,
	redFilter []int,
	minStart int,
) ([]int, bool) {
	block := toSet(redFilter)
	cands := make([]int, 0, 33)
	for n := max(1, minStart); n <= 33; n++ {
		if _, bad := block[n]; !bad {
			cands = append(cands, n)
		}
	}
	if len(cands) < 6 {
		return nil, false
	}

	// 随机多次尝试
	for tries := 0; tries < 5000; tries++ {
		r.Shuffle(len(cands), func(i, j int) { cands[i], cands[j] = cands[j], cands[i] })
		red := append([]int(nil), cands[:6]...)
		sort.Ints(red)
		if _, used := redHistory[redKeyStr(red)]; !used {
			for _, v := range red {
				currFreq[v]++
			}
			return red, true
		}
	}

	// 确定性冷号拼接 + 修补
	type pair struct{ n, f int }
	all := make([]pair, 0, len(cands))
	for _, n := range cands {
		all = append(all, pair{n, histFreq[n] + currFreq[n]})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].f == all[j].f {
			return all[i].n < all[j].n
		}
		return all[i].f < all[j].f
	})
	if len(all) < 6 {
		return nil, false
	}
	red := []int{all[0].n, all[1].n, all[2].n, all[3].n, all[4].n, all[5].n}
	sort.Ints(red)
	if _, used := redHistory[redKeyStr(red)]; !used {
		for _, v := range red {
			currFreq[v]++
		}
		return red, true
	}
	for pos := 5; pos >= 0; pos-- {
		for j := 6; j < len(all); j++ {
			red[pos] = all[j].n
			tmp := append([]int(nil), red...)
			sort.Ints(tmp)
			if _, used := redHistory[redKeyStr(tmp)]; !used {
				for _, v := range tmp {
					currFreq[v]++
				}
				return tmp, true
			}
		}
		red[pos] = all[pos].n
	}
	return nil, false
}

func respectOverlap(trial []int, existing [][6]int, maxOverlap int) bool {
	set := make(map[int]struct{}, len(trial))
	for _, v := range trial {
		set[v] = struct{}{}
	}
	for _, comb := range existing {
		over := 0
		for _, v := range comb {
			if _, ok := set[v]; ok {
				over++
				if over > maxOverlap {
					return false
				}
			}
		}
	}
	return true
}

func readHistoryAndNextRowWithFreq(f *excelize.File) (map[string]struct{}, [34]int, int) {
	rows, err := f.GetRows(sheetName)
	if err != nil {
		log.Fatalf("读取 Excel 行失败: %v", err)
	}
	history := make(map[string]struct{}, 4096)
	var freq [34]int
	next := 2
	foundEmpty := false

	for i, row := range rows {
		if i == 0 {
			continue
		}
		if len(row) < 3 || row[2] == "" {
			next = i + 1
			foundEmpty = true
			break
		}
		nums := strings.Split(row[2], "\n")
		if len(nums) != 7 {
			continue
		}
		reds := make([]int, 0, 6)
		for j := 0; j < 6; j++ {
			if v, err := strconv.Atoi(nums[j]); err == nil {
				reds = append(reds, v)
				if 1 <= v && v <= 33 {
					freq[v]++
				}
			}
		}
		sort.Ints(reds)
		history[redKeyStr(reds)] = struct{}{}
		next = i + 2
	}
	if !foundEmpty && len(rows) > 0 {
		next = len(rows) + 1
	}
	return history, freq, next
}

/* =============================== Lucky / Blue / Utils =============================== */

func buildLuckyList(cfg Config) []int {
	m := map[int]struct{}{}
	for _, n := range cfg.FixedRed {
		if 1 <= n && n <= 33 {
			m[n] = struct{}{}
		}
	}
	if (cfg.Mode == ModeZodiac || cfg.Mode == ModeMixed) && cfg.Animal >= Rat && cfg.Animal <= Pig {
		m[int(cfg.Animal)] = struct{}{}
	}
	if (cfg.Mode == ModeBirthday || cfg.Mode == ModeMixed) && cfg.Birthday != "" {
		if y, mm, dd, ok := parseBirthday(cfg.Birthday); ok {
			for _, n := range []int{mm, dd, digitSum(y)} {
				if 1 <= n && n <= 33 {
					m[n] = struct{}{}
				}
			}
		}
	}
	out := make([]int, 0, len(m))
	for n := range m {
		out = append(out, n)
	}
	sort.Ints(out)
	return out
}

func buildAvailableBlues(blueFilter []int) []int {
	block := toSet(blueFilter)
	out := make([]int, 0, 16)
	for b := 1; b <= 16; b++ {
		if _, bad := block[b]; !bad {
			out = append(out, b)
		}
	}
	return out
}

func roundRobin(base []int, n, offset int) []int {
	L := len(base)
	res := make([]int, n)
	off := ((offset % L) + L) % L
	for i := 0; i < n; i++ {
		res[i] = base[(off+i)%L]
	}
	return res
}

func stableSeedFromBirthday(y, m, d int) int {
	base := y*10000 + m*100 + d
	return base*131 + digitSum(y+m+d)*17
}

func redKeyStr(red []int) string {
	sb := strings.Builder{}
	for i, v := range red {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(fmt.Sprintf("%02d", v))
	}
	return sb.String()
}

/* =============================== 小工具 =============================== */

func toSet(nums []int) map[int]struct{} {
	m := make(map[int]struct{}, len(nums))
	for _, n := range nums {
		m[n] = struct{}{}
	}
	return m
}
func contains(a []int, x int) bool {
	for _, v := range a {
		if v == x {
			return true
		}
	}
	return false
}
func toFixedArray6(a []int) [6]int {
	var r [6]int
	copy(r[:], a)
	return r
}
func parseBirthday(s string) (int, int, int, bool) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return 0, 0, 0, false
	}
	return t.Year(), int(t.Month()), t.Day(), true
}
func digitSum(n int) int {
	sum := 0
	for n > 0 {
		sum += n % 10
		n /= 10
	}
	if sum < 1 {
		sum = 1
	}
	if sum > 33 {
		sum = (sum-1)%33 + 1
	}
	return sum
}

// 每注使用多少幸运号：FixedAlways=全部；FixedRotate=轮转取 FixedPerTicket
func pickLuckyForTicket(cfg Config, luckyAll []int, ticketIdx int) []int {
	if len(luckyAll) == 0 {
		return nil
	}
	switch cfg.FMode {
	case FixedAlways:
		if len(luckyAll) > 6 {
			return luckyAll[:6]
		}
		return luckyAll
	case FixedRotate:
		k := cfg.FixedPerTicket
		if k <= 0 {
			return nil
		}
		if k > 6 {
			k = 6
		}
		if len(luckyAll) <= k {
			// 总数不超过 k，直接全给
			return luckyAll
		}
		out := make([]int, 0, k)
		start := ((ticketIdx % len(luckyAll)) + len(luckyAll)) % len(luckyAll)
		for i := 0; i < k; i++ {
			out = append(out, luckyAll[(start+i)%len(luckyAll)])
		}
		return out
	default:
		return nil
	}
}
