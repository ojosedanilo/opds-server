package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ─── Config ────────────────────────────────────────────────────────────────────

const (
	serverTitle = "Minha Biblioteca OPDS"
	serverID    = "urn:uuid:opds-library-server"
)

var booksDir = "./books"

// ─── OPDS / Atom structs ───────────────────────────────────────────────────────

type Feed struct {
	XMLName    xml.Name `xml:"feed"`
	XMLNS      string   `xml:"xmlns,attr"`
	DC         string   `xml:"xmlns:dc,attr"`
	OpenSearch string   `xml:"xmlns:opensearch,attr"`
	OPDS       string   `xml:"xmlns:opds,attr"`
	ID         string   `xml:"id"`
	Title      string   `xml:"title"`
	Updated    string   `xml:"updated"`
	Links      []Link   `xml:"link"`
	Entries    []Entry  `xml:"entry"`
}

type Entry struct {
	Title   string `xml:"title"`
	ID      string `xml:"id"`
	Updated string `xml:"updated"`
	Summary string `xml:"summary,omitempty"`
	Links   []Link `xml:"link"`
}

type Link struct {
	Rel   string `xml:"rel,attr,omitempty"`
	Href  string `xml:"href,attr"`
	Type  string `xml:"type,attr,omitempty"`
	Title string `xml:"title,attr,omitempty"`
}

// ─── Book ──────────────────────────────────────────────────────────────────────

type Book struct {
	Filename string
	Title    string
	MimeType string
	ModTime  time.Time
}

func extensionMime(ext string) string {
	switch ext {
	case ".epub":
		return "application/epub+zip"
	case ".pdf":
		return "application/pdf"
	case ".mobi":
		return "application/x-mobipocket-ebook"
	case ".cbz":
		return "application/x-cbz"
	case ".cbr":
		return "application/x-cbr"
	default:
		return mime.TypeByExtension(ext)
	}
}

func scanBooks() ([]Book, error) {
	var books []Book
	entries, err := os.ReadDir(booksDir)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler pasta de livros: %w", err)
	}
	supported := map[string]bool{
		".pdf": true, ".epub": true, ".mobi": true, ".cbz": true, ".cbr": true,
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if !supported[ext] {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		title := strings.TrimSuffix(name, filepath.Ext(name))
		title = strings.ReplaceAll(title, "_", " ")
		title = strings.ReplaceAll(title, "-", " ")
		books = append(books, Book{
			Filename: name,
			Title:    title,
			MimeType: extensionMime(ext),
			ModTime:  info.ModTime(),
		})
	}
	sort.Slice(books, func(i, j int) bool {
		return strings.ToLower(books[i].Title) < strings.ToLower(books[j].Title)
	})
	return books, nil
}

// ─── Helpers ───────────────────────────────────────────────────────────────────

// detectScheme determina o esquema (http/https) de forma totalmente automática.
// Ordem de prioridade:
//  1. Cabeçalho "Forwarded" (RFC 7239) — padrão moderno
//  2. "X-Forwarded-Proto" — Cloudflare, nginx, AWS ALB, Traefik…
//  3. "X-Forwarded-Ssl: on" — alguns proxies mais antigos
//  4. TLS nativo — quando o Go mesmo termina o TLS
//  5. Fallback: http
func detectScheme(r *http.Request) string {
	// 1. Forwarded: proto=https (RFC 7239)
	if fwd := r.Header.Get("Forwarded"); fwd != "" {
		for _, part := range strings.Split(fwd, ";") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(strings.ToLower(part), "proto=") {
				return strings.TrimPrefix(strings.ToLower(part), "proto=")
			}
		}
	}
	// 2. X-Forwarded-Proto (Cloudflare Tunnel, nginx, etc.)
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return strings.ToLower(strings.SplitN(proto, ",", 2)[0])
	}
	// 3. X-Forwarded-Ssl: on
	if r.Header.Get("X-Forwarded-Ssl") == "on" {
		return "https"
	}
	// 4. TLS nativo
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func getBase(r *http.Request) string {
	return detectScheme(r) + "://" + r.Host
}

func writeFeed(w http.ResponseWriter, feed Feed) {
	w.Header().Set("Content-Type", "application/atom+xml;profile=opds-catalog; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	out, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		http.Error(w, "erro ao gerar XML", http.StatusInternalServerError)
		return
	}
	w.Write([]byte(xml.Header))
	w.Write(out)
}

func baseLinks(base, self, selfType string) []Link {
	return []Link{
		{Rel: "self", Href: self, Type: selfType},
		{Rel: "start", Href: base + "/opds", Type: "application/atom+xml;profile=opds-catalog;kind=navigation"},
		{Rel: "search", Href: base + "/opds/search?q={searchTerms}", Type: "application/atom+xml"},
	}
}

// ─── Handlers ──────────────────────────────────────────────────────────────────

func handleRoot(w http.ResponseWriter, r *http.Request) {
	base := getBase(r)
	now := time.Now().UTC().Format(time.RFC3339)
	feed := Feed{
		XMLNS: "http://www.w3.org/2005/Atom",
		DC: "http://purl.org/dc/terms/", OpenSearch: "http://a9.com/-/spec/opensearch/1.1/",
		OPDS:    "http://opds-spec.org/2010/catalog",
		ID:      serverID,
		Title:   serverTitle,
		Updated: now,
		Links:   baseLinks(base, base+"/opds", "application/atom+xml;profile=opds-catalog;kind=navigation"),
		Entries: []Entry{{
			Title:   "Todos os Livros",
			ID:      "urn:opds:all",
			Updated: now,
			Summary: "Todos os livros da biblioteca",
			Links:   []Link{{Rel: "subsection", Href: base + "/opds/books", Type: "application/atom+xml;profile=opds-catalog;kind=acquisition"}},
		}},
	}
	writeFeed(w, feed)
}

func handleBooksList(w http.ResponseWriter, r *http.Request, query string) {
	base := getBase(r)
	books, err := scanBooks()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	var entries []Entry
	for _, b := range books {
		if query != "" && !strings.Contains(strings.ToLower(b.Title), query) {
			continue
		}
		entries = append(entries, Entry{
			Title:   b.Title,
			ID:      "urn:opds:book:" + b.Filename,
			Updated: b.ModTime.UTC().Format(time.RFC3339),
			Links: []Link{{
				Rel:  "http://opds-spec.org/acquisition",
				Href: base + "/books/" + url.PathEscape(b.Filename),
				Type: b.MimeType,
			}},
		})
	}
	title := serverTitle + " – Todos os Livros"
	selfHref := base + "/opds/books"
	if query != "" {
		title = "Resultados para: " + query
		selfHref = base + "/opds/search?q=" + url.QueryEscape(query)
	}
	feed := Feed{
		XMLNS: "http://www.w3.org/2005/Atom",
		DC: "http://purl.org/dc/terms/", OpenSearch: "http://a9.com/-/spec/opensearch/1.1/",
		OPDS:    "http://opds-spec.org/2010/catalog",
		ID:      "urn:opds:books",
		Title:   title,
		Updated: now,
		Links:   append(baseLinks(base, selfHref, "application/atom+xml;profile=opds-catalog;kind=acquisition"), Link{Rel: "up", Href: base + "/opds", Type: "application/atom+xml;profile=opds-catalog;kind=navigation"}),
		Entries: entries,
	}
	writeFeed(w, feed)
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	rawName := strings.TrimPrefix(r.URL.Path, "/books/")
	filename, err := url.PathUnescape(rawName)
	if err != nil || filename == "" || strings.Contains(filename, "..") || strings.ContainsAny(filename, "/\\") {
		http.Error(w, "arquivo inválido", http.StatusBadRequest)
		return
	}
	fullPath := filepath.Join(booksDir, filename)
	absBooks, _ := filepath.Abs(booksDir)
	absFile, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absFile, absBooks+string(os.PathSeparator)) {
		http.Error(w, "acesso negado", http.StatusForbidden)
		return
	}
	f, err := os.Open(fullPath)
	if err != nil {
		http.Error(w, "arquivo não encontrado", http.StatusNotFound)
		return
	}
	defer f.Close()
	mtype := extensionMime(strings.ToLower(filepath.Ext(filename)))
	w.Header().Set("Content-Type", mtype)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	if info, err := f.Stat(); err == nil {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	}
	io.Copy(w, f)
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	books, _ := scanBooks()
	base := getBase(r)
	var sb strings.Builder
	for _, b := range books {
		sb.WriteString(fmt.Sprintf("  <li>%s <small>(%s)</small></li>\n", b.Title, filepath.Ext(b.Filename)))
	}
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="pt-BR">
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s</title>
<style>
  *{box-sizing:border-box;margin:0;padding:0}
  body{font-family:system-ui,sans-serif;background:#f5f5f0;color:#222;padding:40px 20px}
  .card{max-width:640px;margin:0 auto;background:#fff;border-radius:12px;box-shadow:0 2px 12px rgba(0,0,0,.08);padding:36px}
  h1{font-size:1.6rem;margin-bottom:8px}
  .badge{display:inline-block;background:#2563eb;color:#fff;border-radius:20px;padding:2px 12px;font-size:.8rem;margin-bottom:20px}
  .endpoint{background:#f0f4ff;border:1px solid #c7d7fe;border-radius:8px;padding:12px 16px;font-family:monospace;font-size:.9rem;word-break:break-all;margin-bottom:24px}
  .endpoint a{color:#2563eb;text-decoration:none}
  h2{font-size:1rem;font-weight:600;margin-bottom:10px;color:#555}
  ul{padding-left:1.4em;line-height:1.9}
  small{color:#888}
  .footer{margin-top:20px;font-size:.75rem;color:#aaa}
</style></head>
<body>
<div class="card">
  <h1>📚 %s</h1>
  <span class="badge">%d livro(s)</span>
  <h2>Endpoint OPDS — adicione no Readest ou Kybook:</h2>
  <div class="endpoint"><a href="%s/opds">%s/opds</a></div>
  <h2>Livros disponíveis</h2>
  <ul>%s</ul>
  <p class="footer">Pasta: <code>%s</code></p>
</div>
</body></html>`, serverTitle, serverTitle, len(books), base, base, sb.String(), booksDir)
}

// ─── Main ──────────────────────────────────────────────────────────────────────

func main() {
	port := "8080"
	args := os.Args[1:]
	if len(args) >= 1 {
		port = args[0]
	}
	if len(args) >= 2 {
		booksDir = args[1]
	}

	if err := os.MkdirAll(booksDir, 0755); err != nil {
		log.Fatalf("Não foi possível criar pasta de livros: %v", err)
	}
	absDir, _ := filepath.Abs(booksDir)

	log.Printf("📚 Servidor OPDS iniciando...")
	log.Printf("   Pasta de livros : %s", absDir)
	log.Printf("   Página web      : http://localhost:%s", port)
	log.Printf("   Feed OPDS       : http://localhost:%s/opds", port)
	log.Printf("   (esquema público detectado automaticamente via cabeçalhos de proxy)")

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/opds", handleRoot)
	mux.HandleFunc("/opds/books", func(w http.ResponseWriter, r *http.Request) {
		handleBooksList(w, r, "")
	})
	mux.HandleFunc("/opds/search", func(w http.ResponseWriter, r *http.Request) {
		q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
		handleBooksList(w, r, q)
	})
	mux.HandleFunc("/books/", handleDownload)

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
