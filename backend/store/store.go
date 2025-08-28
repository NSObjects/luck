package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3" // 若需无 CGO：改为 _ "modernc.org/sqlite"
	"github.com/xuri/excelize/v2"
)

var ErrAlreadyInitialized = errors.New("already_initialized")

// 与第三方/前端完全一致
type Draw struct {
	Issue     string    `json:"issue"`      // 期号，如 "2024098"
	DrawDate  string    `json:"draw_date"`  // YYYY-MM-DD
	Reds      []int     `json:"reds"`       // 升序 6 个 (1..33)
	Blue      int       `json:"blue"`       // 1..16
	Source    string    `json:"source"`     // crawler/excel/...
	FetchedAt time.Time `json:"fetched_at"` // 服务端获取时间
}

type HistorySummary struct {
	TotalCombos int    `json:"total_combos"`
	TotalRows   int    `json:"total_rows"`
	Initialized bool   `json:"initialized"`
	StorePath   string `json:"store_path"`
}

type Store struct {
	db     *sql.DB
	dbPath string
}

/* --------------------------- Open & Migrate --------------------------- */

func Open(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dataDir, "app.db")

	// mattn 驱动（CGO）。若改用 modernc，请把 "sqlite3" 改为 "sqlite" 并调整 DSN。
	dsn := dbPath + "?_busy_timeout=5000&_foreign_keys=on"

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL;`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db, dbPath: dbPath}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS draws (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  issue      TEXT UNIQUE,           -- 唯一期号（可为空；为空则不唯一）
  draw_date  TEXT NOT NULL,         -- YYYY-MM-DD
  reds       TEXT NOT NULL,         -- JSON 数组，如 "[1,2,3,4,5,6]"
  blue       INTEGER NOT NULL,
  source     TEXT,
  fetched_at TEXT NOT NULL,         -- RFC3339
  created_at TEXT DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);
CREATE INDEX IF NOT EXISTS idx_draws_date ON draws(draw_date DESC, id DESC);
CREATE UNIQUE INDEX IF NOT EXISTS ux_draws_issue ON draws(issue);
`)
	return err
}

/* -------------------------- Import from Excel ------------------------- */

// 初始化/覆盖导入 Excel（sheet1；第1/2行为表头；第2列日期；第3列为 7 行号码）。
// replace=false 且已有数据 → ErrAlreadyInitialized
func (s *Store) ImportExcel(xlsxPath string, replace bool) (int, error) {
	has, err := s.hasAny()
	if err != nil {
		return 0, err
	}
	if has && !replace {
		return 0, ErrAlreadyInitialized
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		} else {
			_ = tx.Commit()
		}
	}()

	if replace {
		if _, err = tx.Exec(`DELETE FROM draws`); err != nil {
			return 0, err
		}
	}

	f, err := excelize.OpenFile(xlsxPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	sheet := ""
	for _, name := range f.GetSheetList() {
		if strings.EqualFold(name, "sheet1") {
			sheet = name
			break
		}
	}
	if sheet == "" {
		l := f.GetSheetList()
		if len(l) == 0 {
			return 0, fmt.Errorf("excel 文件无工作表")
		}
		sheet = l[0]
	}

	rows, err := f.GetRows(sheet)
	if err != nil {
		return 0, err
	}

	stmt, err := tx.Prepare(`
INSERT INTO draws(issue, draw_date, reds, blue, source, fetched_at)
VALUES(?,?,?,?,?,?)
`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	imported := 0
	nowStr := time.Now().Format(time.RFC3339Nano)

	for i := 2; i < len(rows); i++ { // 跳过前两行表头
		row := rows[i]
		if len(row) < 3 {
			continue
		}
		issue := strings.TrimSpace(row[0]) // 期号（可空）
		drawDate := normalizeDate(strings.TrimSpace(row[1]))
		codes := strings.TrimSpace(row[2]) // 6 红 + 1 蓝，\n 分行
		if drawDate == "" || codes == "" {
			continue
		}

		parts := strings.Split(codes, "\n")
		if len(parts) != 7 {
			continue
		}
		reds := make([]int, 0, 6)
		ok := true
		for j := 0; j < 6; j++ {
			var n int
			if _, e := fmt.Sscanf(strings.TrimSpace(parts[j]), "%d", &n); e != nil || n < 1 || n > 33 {
				ok = false
				break
			}
			reds = append(reds, n)
		}
		if !ok {
			continue
		}
		sort.Ints(reds)
		var blue int
		if _, e := fmt.Sscanf(strings.TrimSpace(parts[6]), "%d", &blue); e != nil || blue < 1 || blue > 16 {
			continue
		}

		redsJSON, _ := json.Marshal(reds)
		if _, err := stmt.Exec(nullIfEmpty(issue), drawDate, string(redsJSON), blue, "excel", nowStr); err != nil {
			return imported, err
		}
		imported++
	}

	return imported, nil
}

/* -------------------------------- Queries ----------------------------- */

func (s *Store) hasAny() (bool, error) {
	var x int
	err := s.db.QueryRow(`SELECT 1 FROM draws LIMIT 1`).Scan(&x)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

// Upsert：按 issue 幂等；issue 为空则退化为插入（可能存在多行）
func (s *Store) UpsertDrawByIssue(d Draw) error {
	norm, err := normalizeDraw(d)
	if err != nil {
		return err
	}
	redsJSON, _ := json.Marshal(norm.Reds)
	ts := norm.FetchedAt
	if ts.IsZero() {
		ts = time.Now()
	}
	tsStr := ts.Format(time.RFC3339Nano)

	issue := strings.TrimSpace(norm.Issue)
	if issue == "" {
		_, err := s.db.Exec(`
INSERT INTO draws(issue, draw_date, reds, blue, source, fetched_at)
VALUES(NULL,?,?,?,?,?)
`, norm.DrawDate, string(redsJSON), norm.Blue, norm.Source, tsStr)
		return err
	}

	_, err = s.db.Exec(`
INSERT INTO draws(issue, draw_date, reds, blue, source, fetched_at)
VALUES(?,?,?,?,?,?)
ON CONFLICT(issue) DO UPDATE SET
  draw_date = excluded.draw_date,
  reds      = excluded.reds,
  blue      = excluded.blue,
  source    = excluded.source,
  fetched_at= excluded.fetched_at
`, issue, norm.DrawDate, string(redsJSON), norm.Blue, norm.Source, tsStr)
	return err
}

// ReconcileIssue：比较后决定 inserted/updated/noop；返回旧值（若存在）
func (s *Store) ReconcileIssue(in Draw) (status string, prev *Draw, err error) {
	norm, err := normalizeDraw(in)
	if err != nil {
		return "", nil, err
	}
	old, err := s.GetByIssue(norm.Issue)
	if err != nil {
		return "", nil, err
	}
	if old == nil {
		if err := s.UpsertDrawByIssue(norm); err != nil {
			return "", nil, err
		}
		return "inserted", nil, nil
	}
	same := old.DrawDate == norm.DrawDate &&
		old.Blue == norm.Blue &&
		len(old.Reds) == 6 &&
		old.Reds[0] == norm.Reds[0] && old.Reds[1] == norm.Reds[1] &&
		old.Reds[2] == norm.Reds[2] && old.Reds[3] == norm.Reds[3] &&
		old.Reds[4] == norm.Reds[4] && old.Reds[5] == norm.Reds[5]
	if same {
		return "noop", old, nil
	}
	if err := s.UpsertDrawByIssue(norm); err != nil {
		return "", nil, err
	}
	return "updated", old, nil
}

func (s *Store) GetByIssue(issue string) (*Draw, error) {
	issue = strings.TrimSpace(issue)
	if issue == "" {
		return nil, nil
	}
	row := s.db.QueryRow(`SELECT issue, draw_date, reds, blue, source, fetched_at
FROM draws WHERE issue=? LIMIT 1`, issue)
	var out Draw
	var redsJSON string
	var fetched string
	if err := row.Scan(&out.Issue, &out.DrawDate, &redsJSON, &out.Blue, &out.Source, &fetched); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	var reds []int
	_ = json.Unmarshal([]byte(redsJSON), &reds)
	out.Reds = reds
	if t, e := parseTimeFlexible(fetched); e == nil {
		out.FetchedAt = t
	}
	return &out, nil
}

func (s *Store) LatestDraw() (*Draw, error) {
	row := s.db.QueryRow(`SELECT issue, draw_date, reds, blue, source, fetched_at
FROM draws ORDER BY draw_date DESC, id DESC LIMIT 1`)
	var out Draw
	var redsJSON string
	var fetched string
	if err := row.Scan(&out.Issue, &out.DrawDate, &redsJSON, &out.Blue, &out.Source, &fetched); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	_ = json.Unmarshal([]byte(redsJSON), &out.Reds)
	if t, e := parseTimeFlexible(fetched); e == nil {
		out.FetchedAt = t
	}
	return &out, nil
}

// limit<=0 → 全量（升序）；否则取最近 N 期后再按时间升序返回
func (s *Store) ListRecentDraws(limit int) ([]Draw, error) {
	if limit <= 0 {
		rows, err := s.db.Query(`SELECT issue, draw_date, reds, blue, source, fetched_at
FROM draws ORDER BY draw_date ASC, id ASC`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return scanRows(rows)
	}
	rows, err := s.db.Query(`SELECT issue, draw_date, reds, blue, source, fetched_at FROM (
  SELECT issue, draw_date, reds, blue, source, fetched_at, id
  FROM draws ORDER BY draw_date DESC, id DESC LIMIT ?
) t ORDER BY draw_date ASC, id ASC`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

func scanRows(rows *sql.Rows) ([]Draw, error) {
	var list []Draw
	for rows.Next() {
		var d Draw
		var redsJSON string
		var fetched string
		if err := rows.Scan(&d.Issue, &d.DrawDate, &redsJSON, &d.Blue, &d.Source, &fetched); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(redsJSON), &d.Reds)
		if t, e := parseTimeFlexible(fetched); e == nil {
			d.FetchedAt = t
		}
		list = append(list, d)
	}
	return list, nil
}

/* ------------------- 历史集合 & 频次（供生成器用） ------------------- */

func (s *Store) HistorySetAndFreq() (map[string]struct{}, [34]int, error) {
	set := make(map[string]struct{}, 4096)
	var freq [34]int

	rows, err := s.db.Query(`SELECT reds FROM draws`)
	if err != nil {
		return nil, freq, err
	}
	defer rows.Close()
	for rows.Next() {
		var redsJSON string
		if err := rows.Scan(&redsJSON); err != nil {
			return nil, freq, err
		}
		var reds []int
		if e := json.Unmarshal([]byte(redsJSON), &reds); e != nil || len(reds) != 6 {
			continue
		}
		sort.Ints(reds)
		set[redKey(reds)] = struct{}{}
		for _, v := range reds {
			if v >= 1 && v <= 33 {
				freq[v]++
			}
		}
	}
	return set, freq, nil
}

func (s *Store) HistorySummary() (*HistorySummary, error) {
	var rowsCnt int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM draws`).Scan(&rowsCnt); err != nil {
		return nil, err
	}
	// 计算不同红球组合数量（不建派生列，现场计算）
	rows, err := s.db.Query(`SELECT reds FROM draws`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]struct{}{}
	for rows.Next() {
		var redsJSON string
		if err := rows.Scan(&redsJSON); err != nil {
			return nil, err
		}
		var reds []int
		if e := json.Unmarshal([]byte(redsJSON), &reds); e != nil || len(reds) != 6 {
			continue
		}
		sort.Ints(reds)
		seen[redKey(reds)] = struct{}{}
	}
	return &HistorySummary{
		TotalCombos: len(seen),
		TotalRows:   rowsCnt,
		Initialized: rowsCnt > 0,
		StorePath:   s.dbPath,
	}, nil
}

/* --------------------------------- utils -------------------------------- */

func normalizeDraw(d Draw) (Draw, error) {
	if len(d.DrawDate) >= 10 {
		d.DrawDate = strings.TrimSpace(d.DrawDate[:10])
	} else {
		d.DrawDate = strings.TrimSpace(d.DrawDate)
	}
	if len(d.Reds) != 6 {
		return d, fmt.Errorf("reds length must be 6")
	}
	for _, v := range d.Reds {
		if v < 1 || v > 33 {
			return d, fmt.Errorf("red out of range: %d", v)
		}
	}
	// 这里修正：原来误写成了 if d.Blue < 1 || d > 16
	if d.Blue < 1 || d.Blue > 16 {
		return d, fmt.Errorf("blue out of range: %d", d.Blue)
	}
	// 升序
	cp := append([]int(nil), d.Reds...)
	sort.Ints(cp)
	d.Reds = cp
	return d, nil
}

func normalizeDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// 去括号中的星期
	if i := strings.Index(s, "("); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	s = strings.ReplaceAll(s, "/", "-")
	layouts := []string{"2006-01-02", "2006-1-2", "06-01-02", "2006.01.02"}
	for _, ly := range layouts {
		if t, err := time.Parse(ly, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func parseTimeFlexible(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	for _, ly := range []string{
		time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(ly, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time: %s", s)
}

func redKey(reds []int) string {
	return fmt.Sprintf("%02d,%02d,%02d,%02d,%02d,%02d", reds[0], reds[1], reds[2], reds[3], reds[4], reds[5])
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
