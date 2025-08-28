package generator

import (
	"math"
	"sort"
)

/* ---------- 历史期次模型 ---------- */

type Draw struct {
	Period string // 期号（可空）
	Date   string // YYYY-MM-DD（可空）
	Reds   []int  // 长度=6，升序
	Blue   int
}
type Series struct{ List []Draw }

func (s Series) Window(n int) Series {
	if n <= 0 || n >= len(s.List) {
		return s
	}
	return Series{List: s.List[len(s.List)-n:]}
}

/* ---------- 热冷&间隔 ---------- */

type HotCold struct {
	RedFreq    map[int]int     `json:"redFreq"`
	BlueFreq   map[int]int     `json:"blueFreq"`
	TopHotRed  [][2]int        `json:"topHotRed"` // {号码, 频次}
	TopColdRed [][2]int        `json:"topColdRed"`
	AvgGapRed  map[int]float64 `json:"avgGapRed"`
	MaxGapRed  map[int]int     `json:"maxGapRed"`
	MA33       []float64       `json:"MA33"` // 示例：对1..33频次做移动平均
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

	// 简单移动平均（示例：对1..33的频次序列做窗口=5的MA）
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

/* ---------- 热力图 ---------- */

type Heatmap struct {
	RedMatrix  [][]int `json:"redMatrix"` // 33 x N；1=命中
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

/* ---------- 汇总（奇偶/大小/三区/和值/连号/卡方/熵） ---------- */

type Summary struct {
	Odd, Even              int         `json:"odd"`
	Low, High              int         `json:"low"`
	Area                   [3]int      `json:"area"` // 1-11, 12-22, 23-33 的累计命中
	SumMin, SumMax, SumAvg float64     `json:"sumMin","sumMax","sumAvg"`
	ConsecLenDist          map[int]int `json:"consecLenDist"`
	ChiSquare              float64     `json:"chiSquare"`
	Entropy                float64     `json:"entropy"`
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

		// 连号长度统计
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

	// 卡方 / 熵（红球 1..33）
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

// Go 1.24 有内置 min/max；若你更早版本，可自己实现
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
