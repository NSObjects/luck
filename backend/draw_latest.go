package main

//
//// 返回给前端的开奖数据结构（可按需再扩展）
//type Draw struct {
//	Issue     string    `json:"issue"`     // 期号，如 "2024098"
//	DrawDate  string    `json:"draw_date"` // 开奖日期 "YYYY-MM-DD"
//	Reds      []int     `json:"reds"`      // 6 个红球，严格递增，范围 1..33
//	Blue      int       `json:"blue"`      // 1 个蓝球，范围 1..16
//	Source    string    `json:"source,omitempty"`
//	FetchedAt time.Time `json:"fetched_at"` // 服务端获取时间
//}
//
/////* 历史开奖结构（给分析用） */
////type Draw struct {
////	Period string
////	Date   string // YYYY-MM-DD
////	Reds   []int  // 升序 6 个
////	Blue   int
////}
//
//// 在你的 gin.Engine 初始化后调用一次：registerDrawRoutes(r)
//func registerDrawRoutes(r *gin.Engine) {
//	api := r.Group("/api")
//	api.GET("/draw/latest", handleLatestDraw)
//}
//
//func handleLatestDraw(c *gin.Context) {
//	ctx := c.Request.Context()
//
//	// 交给你实现的数据获取逻辑
//	d, err := FetchLatestDraw(ctx)
//	if err != nil {
//		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
//		return
//	}
//	if d == nil {
//		c.JSON(http.StatusNotFound, gin.H{"error": "latest draw not found"})
//		return
//	}
//	if err := validateDraw(*d); err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid draw: " + err.Error()})
//		return
//	}
//	if d.FetchedAt.IsZero() {
//		d.FetchedAt = time.Now()
//	}
//	c.JSON(http.StatusOK, d)
//}
//
//// 你来实现：可从官方 API / 网页解析 / DB 读取等任意来源
//var FetchLatestDraw = func(ctx context.Context) (*Draw, error) {
//	client := resty.New()
//	var l luck
//	_, err := client.R().SetResult(&l).SetQueryParams(map[string]string{
//		"code":       "ssq",
//		"app_id":     "rcdrixmfzrmxho3s",
//		"app_secret": "dkJQRXNlS0dscW44cFZTbnVlbUVvdz09",
//	}).Get("https://www.mxnzp.com/api/lottery/common/latest")
//	if err != nil {
//		panic(err)
//	}
//	if l.Code != 1 {
//		return nil, fmt.Errorf("invalid response code: %d", l.Code)
//	}
//	red, blue, err := ParseLineInts(l.Data.OpenCode)
//	if err != nil {
//		panic(err)
//	}
//
//	return &Draw{
//		Issue:    l.Data.Expect,
//		DrawDate: l.Data.Time,
//		Reds:     red,
//		Blue:     blue,
//		Source:   "crawler",
//	}, nil
//}
//
//func ParseLineInts(s string) ([]int, int, error) {
//	s = strings.TrimSpace(s)
//	left, right, ok := strings.Cut(s, "+")
//	if !ok {
//		return nil, 0, fmt.Errorf("missing '+' part")
//	}
//
//	// 解析主数组
//	var red []int
//	for i, part := range strings.Split(left, ",") {
//		n, err := strconv.Atoi(strings.TrimSpace(part))
//		if err != nil {
//			return nil, 0, fmt.Errorf("invalid number at index %d: %v", i, err)
//		}
//		red = append(red, n)
//	}
//
//	// 解析加号后的数字（丢前导 0）
//	blue, err := strconv.Atoi(strings.TrimSpace(right))
//	if err != nil {
//		return nil, 0, fmt.Errorf("invalid plus number: %v", err)
//	}
//
//	return red, blue, nil
//}
//
//func validateDraw(d Draw) error {
//	if len(d.Reds) != 6 {
//		return fmt.Errorf("reds must be 6 numbers")
//	}
//	last := 0
//	for _, v := range d.Reds {
//		if v < 1 || v > 33 {
//			return fmt.Errorf("red out of range: %d", v)
//		}
//		if v <= last {
//			return fmt.Errorf("reds must be strictly increasing")
//		}
//		last = v
//	}
//	if d.Blue < 1 || d.Blue > 16 {
//		return fmt.Errorf("blue out of range: %d", d.Blue)
//	}
//	return nil
//}
//
//type luck struct {
//	Code int    `json:"code"`
//	Msg  string `json:"msg"`
//	Data struct {
//		OpenCode string `json:"openCode"`
//		Code     string `json:"code"`
//		Expect   string `json:"expect"`
//		Name     string `json:"name"`
//		Time     string `json:"time"`
//	} `json:"data"`
//}
