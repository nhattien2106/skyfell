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
	   ID            int      `json:"id"`
	   URL           string   `json:"url"`
	   Title         string   `json:"title"`
	   Meta          string   `json:"meta"`
	   InternalLinks int      `json:"internal_links"`
	   ExternalLinks int      `json:"external_links"`
	   BrokenLinks   int      `json:"broken_links"`
	   BrokenList    []string `json:"broken_list"`
	   Status        string   `json:"status"`
}


func main() {
	   db, err := sql.Open("mysql", "admin:admin@tcp(localhost:3306)/skyfell")
	   if err != nil {
			   log.Fatal(err)
	   }
	   defer db.Close()

	   r := gin.Default()

	   // CORS middleware
	   r.Use(func(c *gin.Context) {
			   c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
			   c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			   c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
			   c.Writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
			   if c.Request.Method == "OPTIONS" {
					   c.AbortWithStatus(204)
					   return
			   }
			   c.Next()
	   })

	   // POST /api/crawl (re-crawl or new crawl)
	   r.POST("/api/crawl", func(c *gin.Context) {
			   var req struct {
					   URL string `json:"url"`
			   }
			   if err := c.ShouldBindJSON(&req); err != nil {
					   c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
					   return
			   }

			   // Set status to queued
			   _, _ = db.Exec("UPDATE pages SET status='queued' WHERE url=?", req.URL)

			   // Simulate real-time status (in production, use goroutine/worker)
			   status := "running"
			   _, _ = db.Exec("UPDATE pages SET status=? WHERE url=?", status, req.URL)

			   // Fetch and parse the page
			   resp, err := http.Get(req.URL)
			   if err != nil {
					   status = "error"
					   _, _ = db.Exec("UPDATE pages SET status=? WHERE url=?", status, req.URL)
					   c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to fetch URL"})
					   return
			   }
			   defer resp.Body.Close()

			   z := html.NewTokenizer(resp.Body)
			   var title, meta string
			   var internalLinks, externalLinks, brokenLinks int
			   var brokenList []string
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
							   if t.Data == "a" {
									   var href string
									   for _, attr := range t.Attr {
											   if attr.Key == "href" {
													   href = attr.Val
											   }
									   }
									   if href != "" {
											   if strings.HasPrefix(href, req.URL) || strings.HasPrefix(href, "/") {
													   internalLinks++
											   } else if strings.HasPrefix(href, "http") {
													   externalLinks++
													   // Check if link is broken
													   resp2, err2 := http.Head(href)
													   if err2 != nil || resp2.StatusCode >= 400 {
															   brokenLinks++
															   brokenList = append(brokenList, href)
													   }
													   if resp2 != nil {
															   resp2.Body.Close()
													   }
											   }
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

			   status = "done"
			   // Store in DB (upsert)
			   _, err = db.Exec(`INSERT INTO pages (url, title, meta, internal_links, external_links, broken_links, broken_list, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE title=VALUES(title), meta=VALUES(meta), internal_links=VALUES(internal_links), external_links=VALUES(external_links), broken_links=VALUES(broken_links), broken_list=VALUES(broken_list), status=VALUES(status)`, req.URL, title, meta, internalLinks, externalLinks, brokenLinks, strings.Join(brokenList, ","), status)
			   if err != nil {
					   c.JSON(http.StatusInternalServerError, gin.H{"error": "DB error"})
					   return
			   }

			   c.JSON(http.StatusOK, gin.H{"url": req.URL, "title": title, "meta": meta, "internal_links": internalLinks, "external_links": externalLinks, "broken_links": brokenLinks, "broken_list": brokenList, "status": status})
	   })

	   r.GET("/api/pages", func(c *gin.Context) {
			   rows, err := db.Query("SELECT id, url, title, meta, internal_links, external_links, broken_links, broken_list, status FROM pages")
			   if err != nil {
					   c.JSON(http.StatusInternalServerError, gin.H{"error": "DB error"})
					   return
			   }
			   defer rows.Close()

			   var pages []PageInfo
			   for rows.Next() {
					   var p PageInfo
					   var brokenListStr string
					   if err := rows.Scan(&p.ID, &p.URL, &p.Title, &p.Meta, &p.InternalLinks, &p.ExternalLinks, &p.BrokenLinks, &brokenListStr, &p.Status); err != nil {
							   continue
					   }
					   if brokenListStr != "" {
							   p.BrokenList = strings.Split(brokenListStr, ",")
					   }
					   pages = append(pages, p)
			   }
			   c.JSON(http.StatusOK, pages)
	   })

	   // GET /api/pages/:id (details view)
	   r.GET("/api/pages/:id", func(c *gin.Context) {
			   id := c.Param("id")
			   var p PageInfo
			   var brokenListStr string
			   err := db.QueryRow("SELECT id, url, title, meta, internal_links, external_links, broken_links, broken_list, status FROM pages WHERE id = ?", id).Scan(&p.ID, &p.URL, &p.Title, &p.Meta, &p.InternalLinks, &p.ExternalLinks, &p.BrokenLinks, &brokenListStr, &p.Status)
			   if err != nil {
					   c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
					   return
			   }
			   if brokenListStr != "" {
					   p.BrokenList = strings.Split(brokenListStr, ",")
			   }
			   c.JSON(http.StatusOK, p)
	   })

	   // DELETE /api/pages (bulk delete)
	   r.DELETE("/api/pages", func(c *gin.Context) {
			   var req struct {
					   IDs []int `json:"ids"`
			   }
			   if err := c.ShouldBindJSON(&req); err != nil {
					   c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
					   return
			   }
			   for _, id := range req.IDs {
					   _, _ = db.Exec("DELETE FROM pages WHERE id = ?", id)
			   }
			   c.JSON(http.StatusOK, gin.H{"deleted": req.IDs})
	   })

	r.Run(":8080")
}
