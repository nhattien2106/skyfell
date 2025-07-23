package main

import (
	"database/sql"
	"log"
	"net/http"
	"net/url"
	"strings"
	"regexp"
	"io"
	"encoding/json"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/net/html"
)

type PageInfo struct {
	ID                int               `json:"id"`
	URL               string            `json:"url"`
	Title             string            `json:"title"`
	Meta              string            `json:"meta"`
	HTMLVersion       string            `json:"htmlVersion"`
	Headings          map[string]int    `json:"headings"`
	InternalLinks     int               `json:"internalLinks"`
	ExternalLinks     int               `json:"externalLinks"`
	BrokenLinks       int               `json:"brokenLinks"`
	LoginForm         bool              `json:"loginForm"`
	BrokenLinkDetails []BrokenLinkInfo  `json:"brokenLinkDetails"`
}

type BrokenLinkInfo struct {
	Href   string `json:"href"`
	Status int    `json:"status"`
}

func main() {
	db, err := sql.Open("mysql", "admin:admin@tcp(localhost:3306)/skyfell")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()


   r := gin.Default()

   // CORS middleware for development
   r.Use(func(c *gin.Context) {
	   c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	   c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	   c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept")
	   if c.Request.Method == "OPTIONS" {
		   c.AbortWithStatus(204)
		   return
	   }
	   c.Next()
   })

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

		// Read the body for HTML version detection
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read body"})
			return
		}
		bodyStr := string(bodyBytes)

		// HTML version detection
		htmlVersion := "Unknown"
		reDoctype := regexp.MustCompile(`(?i)<!DOCTYPE ([^>]+)>`)
		if m := reDoctype.FindStringSubmatch(bodyStr); m != nil {
			doctype := strings.ToLower(m[1])
			switch {
			case strings.Contains(doctype, "html") && !strings.Contains(doctype, "public"):
				htmlVersion = "HTML5"
			case strings.Contains(doctype, "xhtml"):
				htmlVersion = "XHTML"
			case strings.Contains(doctype, "4.01"):
				htmlVersion = "HTML 4.01"
			case strings.Contains(doctype, "3.2"):
				htmlVersion = "HTML 3.2"
			case strings.Contains(doctype, "2.0"):
				htmlVersion = "HTML 2.0"
			default:
				htmlVersion = m[1]
			}
		}

		// Parse HTML
		doc, err := html.Parse(strings.NewReader(bodyStr))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse HTML"})
			return
		}

		// Data to collect
		title := ""
		meta := ""
		headings := map[string]int{"h1": 0, "h2": 0, "h3": 0, "h4": 0, "h5": 0, "h6": 0}
		internalLinks := 0
		externalLinks := 0
		brokenLinks := 0
		brokenLinkDetails := []BrokenLinkInfo{}
		loginForm := false

		// Parse base URL
		parsedBase, _ := url.Parse(req.URL)

		var f func(*html.Node)
		f = func(n *html.Node) {
			if n.Type == html.ElementNode {
				tag := strings.ToLower(n.Data)
				if tag == "title" && n.FirstChild != nil {
					title = n.FirstChild.Data
				}
				if tag == "meta" {
					var name, content string
					for _, attr := range n.Attr {
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
				if tag == "form" {
					for _, attr := range n.Attr {
						if attr.Key == "action" && strings.Contains(strings.ToLower(attr.Val), "login") {
							loginForm = true
						}
					}
				}
				if tag == "input" {
					for _, attr := range n.Attr {
						if attr.Key == "type" && attr.Val == "password" {
							loginForm = true
						}
					}
				}
				if _, ok := headings[tag]; ok {
					headings[tag]++
				}
				if tag == "a" {
					href := ""
					for _, attr := range n.Attr {
						if attr.Key == "href" {
							href = attr.Val
							break
						}
					}
					if href != "" && !strings.HasPrefix(href, "#") && !strings.HasPrefix(href, "javascript:") {
						parsedHref, err := url.Parse(href)
						if err == nil {
							if parsedHref.Host == "" || parsedHref.Host == parsedBase.Host {
								internalLinks++
							} else {
								externalLinks++
							}
							// Check for broken links (only http/https)
							if parsedHref.Scheme == "http" || parsedHref.Scheme == "https" {
								checkUrl := href
								if parsedHref.Host == "" {
									checkUrl = parsedBase.Scheme + "://" + parsedBase.Host + href
								}
								resp, err := http.Head(checkUrl)
								if err != nil || (resp.StatusCode >= 400 && resp.StatusCode < 600) {
									brokenLinks++
									status := 0
									if resp != nil {
										status = resp.StatusCode
									}
									brokenLinkDetails = append(brokenLinkDetails, BrokenLinkInfo{Href: checkUrl, Status: status})
								}
								if resp != nil {
									resp.Body.Close()
								}
							}
						}
					}
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				f(c)
			}
		}
		f(doc)

		if title == "" {
			title = "(no title found)"
		}
		if meta == "" {
			meta = "(no meta description found)"
		}

		// Store in DB (all fields)
		brokenLinkDetailsJSON := ""
		{
			b, err := json.Marshal(brokenLinkDetails)
			if err == nil {
				brokenLinkDetailsJSON = string(b)
			}
		}
		_, err = db.Exec(`INSERT INTO pages 
			(url, title, meta, html_version, h1, h2, h3, h4, h5, h6, internal_links, external_links, broken_links, login_form, broken_link_details)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			req.URL, title, meta, htmlVersion,
			headings["h1"], headings["h2"], headings["h3"], headings["h4"], headings["h5"], headings["h6"],
			internalLinks, externalLinks, brokenLinks, loginForm, brokenLinkDetailsJSON)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"url": req.URL,
			"title": title,
			"meta": meta,
			"htmlVersion": htmlVersion,
			"headings": headings,
			"internalLinks": internalLinks,
			"externalLinks": externalLinks,
			"brokenLinks": brokenLinks,
			"loginForm": loginForm,
			"brokenLinkDetails": brokenLinkDetails,
		})
	})

	r.GET("/api/pages", func(c *gin.Context) {
		rows, err := db.Query(`SELECT id, url, title, meta, html_version, h1, h2, h3, h4, h5, h6, internal_links, external_links, broken_links, login_form, broken_link_details FROM pages`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB error"})
			return
		}
		defer rows.Close()

		var pages []PageInfo
		for rows.Next() {
			var p PageInfo
			var h1, h2, h3, h4, h5, h6 int
			var htmlVersion string
			var internalLinks, externalLinks, brokenLinks int
			var loginForm bool
			var brokenLinkDetailsJSON string
			if err := rows.Scan(&p.ID, &p.URL, &p.Title, &p.Meta, &htmlVersion, &h1, &h2, &h3, &h4, &h5, &h6, &internalLinks, &externalLinks, &brokenLinks, &loginForm, &brokenLinkDetailsJSON); err != nil {
				continue
			}
			p.HTMLVersion = htmlVersion
			p.Headings = map[string]int{"h1": h1, "h2": h2, "h3": h3, "h4": h4, "h5": h5, "h6": h6}
			p.InternalLinks = internalLinks
			p.ExternalLinks = externalLinks
			p.BrokenLinks = brokenLinks
			p.LoginForm = loginForm
			// Parse brokenLinkDetails JSON
			var brokenLinksArr []BrokenLinkInfo
			if err := json.Unmarshal([]byte(brokenLinkDetailsJSON), &brokenLinksArr); err == nil {
				p.BrokenLinkDetails = brokenLinksArr
			} else {
				p.BrokenLinkDetails = nil
			}
			pages = append(pages, p)
		}
		c.JSON(http.StatusOK, pages)
	})

	r.Run(":8080")
}
