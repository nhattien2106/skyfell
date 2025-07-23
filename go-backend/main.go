package main

import (
	"database/sql"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/net/html"
)

type PageInfo struct {
	ID      int    `json:"id"`
	URL     string `json:"url"`
	Title   string `json:"title"`
	Meta    string `json:"meta"`
}

func main() {
	db, err := sql.Open("mysql", "admin:admin@tcp(localhost:3306)/skyfell")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	r := gin.Default()

	r.POST("/api/crawl", func(c *gin.Context) {
		var req struct {
			URL string `json:"url"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		// Fetch and parse the page
		resp, err := http.Get(req.URL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to fetch URL"})
			return
		}
		defer resp.Body.Close()

		z := html.NewTokenizer(resp.Body)
		var title, meta string
		for {
			tt := z.Next()
			switch tt {
			case html.ErrorToken:
				goto done
			case html.StartTagToken, html.SelfClosingTagToken:
				t := z.Token()
				if t.Data == "title" && title == "" {
					z.Next()
					title = strings.TrimSpace(z.Token().Data)
				}
				if t.Data == "meta" {
					var name, content string
					for _, attr := range t.Attr {
						if attr.Key == "name" && attr.Val == "description" {
							name = attr.Val
						}
						if attr.Key == "content" {
							content = attr.Val
						}
					}
					if name == "description" && meta == "" {
						meta = content
					}
				}
			}
		}
	done:
		if title == "" {
			title = "(no title found)"
		}
		if meta == "" {
			meta = "(no meta description found)"
		}

		// Store in DB
		_, err = db.Exec("INSERT INTO pages (url, title, meta) VALUES (?, ?, ?)", req.URL, title, meta)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"url": req.URL, "title": title, "meta": meta})
	})

	r.GET("/api/pages", func(c *gin.Context) {
		rows, err := db.Query("SELECT id, url, title, meta FROM pages")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB error"})
			return
		}
		defer rows.Close()

		var pages []PageInfo
		for rows.Next() {
			var p PageInfo
			if err := rows.Scan(&p.ID, &p.URL, &p.Title, &p.Meta); err != nil {
				continue
			}
			pages = append(pages, p)
		}
		c.JSON(http.StatusOK, pages)
	})

	r.Run(":8080")
}
