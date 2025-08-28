package history

import (
	"luck/backend/store"

	"github.com/gin-gonic/gin"
)

func uploadHistoryHandler(c *gin.Context) {
	st, err := store.Open("data")
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			panic(err)
		}
	}()
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
	st, err := store.Open("data")
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			panic(err)
		}
	}()
	sum, err := st.HistorySummary()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, sum)
}
