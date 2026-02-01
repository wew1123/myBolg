package main

import (
	"bytes"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

type post struct {
	Title   string
	Slug    string
	Summary string
	Date    time.Time
	ModTime time.Time
}

var layout = template.Must(template.New("layout").Parse(`<!doctype html>
<html lang="zh-CN">
  <head>
	<meta charset="utf-8" />
	<meta name="viewport" content="width=device-width, initial-scale=1" />
	<title>{{ .Title }}</title>
	<style>
	  :root { color-scheme: light dark; }
	  body { max-width: 880px; margin: 48px auto; padding: 0 20px; font: 16px/1.7 system-ui, -apple-system, "Segoe UI", sans-serif; }
	  a { text-decoration: none; }
	  header { margin-bottom: 28px; }
	  nav a { margin-right: 12px; }
	  pre { padding: 12px; overflow: auto; border-radius: 8px; background: #f6f8fa; border: 1px solid #e5e7eb; }
	  pre code { display: block; }
	  code { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, "Liberation Mono", monospace; background: #f6f8fa; padding: 2px 4px; border-radius: 4px; }
	  .post-list { list-style: none; padding: 0; }
	  .post-list li { margin: 16px 0; padding-bottom: 12px; border-bottom: 1px solid #ddd; }
	  .post-meta { color: #777; font-size: 14px; }
	  footer { margin-top: 48px; color: #888; font-size: 14px; }
	</style>
  </head>
  <body>
	<header>
	  <nav>
		<a href="/">首页</a>
		<a href="/rss.xml">RSS</a>
	  </nav>
	</header>
	{{ .Content }}
	<footer>© {{ .Year }} 博客</footer>
  </body>
</html>`))

var md = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		highlighting.NewHighlighting(
			highlighting.WithStyle("github"),
		),
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
	),
	goldmark.WithRendererOptions(
		html.WithUnsafe(),
	),
)

func main() {
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/post/", handlePost)
	http.HandleFunc("/rss.xml", handleRSS)
	http.HandleFunc("/sitemap.xml", handleSitemap)

	log.Println("Blog running at http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	posts, err := listPosts("posts")
	if err != nil {
		http.Error(w, "读取文章失败", http.StatusInternalServerError)
		return
	}

	var sb strings.Builder
	sb.WriteString("<h1>博客</h1><ul class=\"post-list\">")
	for _, p := range posts {
		sb.WriteString("<li>")
		sb.WriteString("<a href=\"/post/")
		sb.WriteString(p.Slug)
		sb.WriteString("\">")
		sb.WriteString(template.HTMLEscapeString(p.Title))
		sb.WriteString("</a>")
		sb.WriteString("<div class=\"post-meta\">")
		if !p.Date.IsZero() {
			sb.WriteString(p.Date.Format("2006-01-02"))
		} else if !p.ModTime.IsZero() {
			sb.WriteString(p.ModTime.Format("2006-01-02"))
		}
		sb.WriteString("</div>")
		if p.Summary != "" {
			sb.WriteString("<div>")
			sb.WriteString(template.HTMLEscapeString(p.Summary))
			sb.WriteString("</div>")
		}
		sb.WriteString("</li>")
	}
	sb.WriteString("</ul>")

	renderHTML(w, "首页", template.HTML(sb.String()))
}

func handlePost(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/post/")
	if slug == "" || strings.Contains(slug, "/") {
		http.NotFound(w, r)
		return
	}

	mdPath := filepath.Join("posts", slug+".md")
	b, err := os.ReadFile(mdPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	meta, body := parseFrontMatter(string(b))
	var buf bytes.Buffer
	if err := md.Convert([]byte(body), &buf); err != nil {
		http.Error(w, "渲染失败", http.StatusInternalServerError)
		return
	}

	title := extractTitle(meta, body)
	if title == "" {
		title = slug
	}

	renderHTML(w, title, template.HTML(buf.String()))
}

func renderHTML(w http.ResponseWriter, title string, content template.HTML) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = layout.Execute(w, map[string]any{
		"Title":   title,
		"Content": content,
		"Year":    time.Now().Year(),
	})
}

func listPosts(dir string) ([]post, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, err
	}

	posts := make([]post, 0, len(matches))
	for _, path := range matches {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		info, _ := os.Stat(path)
		slug := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		meta, body := parseFrontMatter(string(b))
		title := extractTitle(meta, body)
		if title == "" {
			title = slug
		}
		p := post{
			Title:   title,
			Slug:    slug,
			Summary: extractSummary(meta, body),
		}
		if info != nil {
			p.ModTime = info.ModTime()
		}
		if dateStr := strings.TrimSpace(meta["date"]); dateStr != "" {
			if t, err := time.Parse("2006-01-02", dateStr); err == nil {
				p.Date = t
			}
		}
		posts = append(posts, p)
	}

	sort.Slice(posts, func(i, j int) bool {
		a := posts[i]
		b := posts[j]
		if !a.Date.IsZero() && !b.Date.IsZero() {
			return a.Date.After(b.Date)
		}
		if !a.Date.IsZero() {
			return true
		}
		if !b.Date.IsZero() {
			return false
		}
		return a.ModTime.After(b.ModTime)
	})

	return posts, nil
}

func extractTitle(meta map[string]string, mdText string) string {
	if v := strings.TrimSpace(meta["title"]); v != "" {
		return v
	}
	for _, line := range strings.Split(mdText, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

func extractSummary(meta map[string]string, mdText string) string {
	if v := strings.TrimSpace(meta["summary"]); v != "" {
		return v
	}
	for _, line := range strings.Split(mdText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "```") {
			continue
		}
		return line
	}
	return ""
}

func parseFrontMatter(mdText string) (map[string]string, string) {
	meta := map[string]string{}
	lines := strings.Split(mdText, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return meta, mdText
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return meta, mdText
	}
	for _, line := range lines[1:end] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])
		meta[key] = val
	}
	body := strings.Join(lines[end+1:], "\n")
	return meta, body
}

func handleRSS(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/rss.xml" {
		http.NotFound(w, r)
		return
	}
	posts, err := listPosts("posts")
	if err != nil {
		http.Error(w, "读取文章失败", http.StatusInternalServerError)
		return
	}

	baseURL := "http://localhost:8080"
	var sb strings.Builder
	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>")
	sb.WriteString("<rss version=\"2.0\"><channel>")
	sb.WriteString("<title>博客</title>")
	sb.WriteString("<link>")
	sb.WriteString(baseURL)
	sb.WriteString("</link>")
	sb.WriteString("<description>个人博客</description>")
	for _, p := range posts {
		url := baseURL + "/post/" + p.Slug
		pub := p.Date
		if pub.IsZero() {
			pub = p.ModTime
		}
		if pub.IsZero() {
			pub = time.Now()
		}
		sb.WriteString("<item>")
		sb.WriteString("<title>")
		sb.WriteString(template.HTMLEscapeString(p.Title))
		sb.WriteString("</title>")
		sb.WriteString("<link>")
		sb.WriteString(url)
		sb.WriteString("</link>")
		sb.WriteString("<guid>")
		sb.WriteString(url)
		sb.WriteString("</guid>")
		sb.WriteString("<pubDate>")
		sb.WriteString(pub.Format(time.RFC1123Z))
		sb.WriteString("</pubDate>")
		if p.Summary != "" {
			sb.WriteString("<description>")
			sb.WriteString(template.HTMLEscapeString(p.Summary))
			sb.WriteString("</description>")
		}
		sb.WriteString("</item>")
	}
	sb.WriteString("</channel></rss>")

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	_, _ = w.Write([]byte(sb.String()))
}

func handleSitemap(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/sitemap.xml" {
		http.NotFound(w, r)
		return
	}
	posts, err := listPosts("posts")
	if err != nil {
		http.Error(w, "读取文章失败", http.StatusInternalServerError)
		return
	}

	baseURL := "http://localhost:8080"
	var sb strings.Builder
	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>")
	sb.WriteString("<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">")
	sb.WriteString("<url><loc>")
	sb.WriteString(baseURL)
	sb.WriteString("</loc></url>")
	for _, p := range posts {
		url := baseURL + "/post/" + p.Slug
		sb.WriteString("<url><loc>")
		sb.WriteString(url)
		sb.WriteString("</loc></url>")
	}
	sb.WriteString("</urlset>")

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = w.Write([]byte(sb.String()))
}
