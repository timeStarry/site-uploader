package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct{ Domain, Token, DataDir, Port, HomeTitle string }
type Site struct {
	ID, Title, Summary, CreatedAt string
	PasswordHash                  string `json:"password_hash,omitempty"`
	PasswordSalt                  string `json:"password_salt,omitempty"`
}
type App struct {
	cfg   Config
	mu    sync.RWMutex
	sites []Site
}

func main() {
	c := Config{env("DOMAIN", "localhost"), os.Getenv("UPLOAD_TOKEN"), env("DATA_DIR", "/data"), env("PORT", "8080"), env("SITE_TITLE", "静态Web托管页面")}
	if c.Token == "" {
		log.Fatal("UPLOAD_TOKEN is required")
	}
	a := &App{cfg: c}
	if e := a.load(); e != nil {
		log.Fatal(e)
	}
	m := http.NewServeMux()
	m.HandleFunc("/api/auth/check", a.authCheck)
	m.HandleFunc("/api/sites", a.sitesAPI)
	m.HandleFunc("/api/sites/search", a.searchAPI)
	m.HandleFunc("/s/", a.serve)
	m.HandleFunc("/", a.home)
	log.Fatal(http.ListenAndServe(":"+c.Port, logging(m)))
}

func (a *App) authCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")) != a.cfg.Token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func (a *App) load() error {
	if err := os.MkdirAll(filepath.Join(a.cfg.DataDir, "html"), 0755); err != nil {
		return err
	}
	b, e := os.ReadFile(filepath.Join(a.cfg.DataDir, "sites.json"))
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	return json.Unmarshal(b, &a.sites)
}
func (a *App) save() error {
	b, _ := json.MarshalIndent(a.sites, "", "  ")
	t := filepath.Join(a.cfg.DataDir, "sites.json.tmp")
	if e := os.WriteFile(t, b, 0644); e != nil {
		return e
	}
	return os.Rename(t, filepath.Join(a.cfg.DataDir, "sites.json"))
}
func (a *App) authorized(r *http.Request) bool {
	return strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")) == a.cfg.Token
}
func (a *App) sitesAPI(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r) {
		http.Error(w, "unauthorized", 401)
		return
	}
	if r.Method == http.MethodGet {
		page, size := pagination(r)
		a.mu.RLock()
		defer a.mu.RUnlock()
		writePage(w, a.sites, page, size, a.cfg.Domain)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if e := r.ParseMultipartForm(20 << 20); e != nil {
		http.Error(w, "invalid form", 400)
		return
	}
	title, summary := strings.TrimSpace(r.FormValue("title")), strings.TrimSpace(r.FormValue("summary"))
	if title == "" {
		http.Error(w, "title is required", 400)
		return
	}
	password := strings.TrimSpace(r.FormValue("access_password"))
	if password != "" && !digits4(password) {
		http.Error(w, "access_password must be exactly four digits", 400)
		return
	}
	f, _, e := r.FormFile("file")
	if e != nil {
		http.Error(w, "file is required", 400)
		return
	}
	defer f.Close()
	b := make([]byte, 8)
	rand.Read(b)
	id := hex.EncodeToString(b)
	p := filepath.Join(a.cfg.DataDir, "html", id+".html")
	o, e := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if e != nil {
		http.Error(w, "storage error", 500)
		return
	}
	_, e = io.Copy(o, io.LimitReader(f, 10<<20))
	o.Close()
	if e != nil {
		os.Remove(p)
		http.Error(w, "upload failed", 500)
		return
	}
	site := Site{ID: id, Title: title, Summary: summary, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	if password != "" {
		site.PasswordSalt = randomHex(16)
		site.PasswordHash = hashPassword(site.PasswordSalt, password)
	}
	a.mu.Lock()
	a.sites = append([]Site{site}, a.sites...)
	e = a.save()
	a.mu.Unlock()
	if e != nil {
		os.Remove(p)
		http.Error(w, "storage error", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"id": id, "url": "https://" + a.cfg.Domain + "/s/" + id, "password_protected": password != ""})
}

func (a *App) searchAPI(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r) {
		http.Error(w, "unauthorized", 401)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		http.Error(w, "q is required", 400)
		return
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	var matches []Site
	for _, site := range a.sites {
		text := strings.ToLower(site.Title + " " + site.Summary + " " + site.ID)
		if fuzzyMatch(text, q) {
			matches = append(matches, site)
		}
	}
	page, size := pagination(r)
	writePage(w, matches, page, size, a.cfg.Domain)
}

func pagination(r *http.Request) (int, int) {
	page, size := 1, 20
	if n, e := strconv.Atoi(r.URL.Query().Get("page")); e == nil && n > 0 {
		page = n
	}
	if n, e := strconv.Atoi(r.URL.Query().Get("page_size")); e == nil && n > 0 {
		size = n
	}
	if size > 100 {
		size = 100
	}
	return page, size
}
func writePage(w http.ResponseWriter, sites []Site, page, size int, domain string) {
	total := len(sites)
	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}
	items := sites[start:end]
	public := make([]map[string]string, 0, len(items))
	for _, site := range items {
		public = append(public, map[string]string{"id": site.ID, "title": site.Title, "summary": site.Summary, "created_at": site.CreatedAt, "url": "https://" + domain + "/s/" + site.ID})
	}
	if public == nil {
		public = []map[string]string{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"items": public, "page": page, "page_size": size, "total": total, "pages": (total + size - 1) / size})
}
func fuzzyMatch(text, query string) bool {
	text, query = strings.ToLower(text), strings.ToLower(query)
	if strings.Contains(text, query) {
		return true
	}
	for _, token := range tokenize(query) {
		if token != "" && !strings.Contains(text, token) {
			return false
		}
	}
	return true
}
func tokenize(s string) []string {
	var out []string
	var b []rune
	flush := func() {
		if len(b) > 0 {
			out = append(out, string(b))
			b = nil
		}
	}
	for _, r := range []rune(strings.ToLower(s)) {
		if r <= 127 && (r == ' ' || r == '\t' || r == '-' || r == '_' || r == '/' || r == '.') {
			flush()
			continue
		}
		if r > 127 {
			flush()
			out = append(out, string(r))
			continue
		}
		b = append(b, r)
	}
	flush()
	return out
}
func digits4(s string) bool {
	if len(s) != 4 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
func randomHex(n int) string {
	b := make([]byte, n)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func hashPassword(salt, password string) string {
	h := sha256.Sum256([]byte(salt + ":" + password))
	return hex.EncodeToString(h[:])
}
func (a *App) passwordOK(site Site, password string) bool {
	return site.PasswordHash == "" || hmac.Equal([]byte(site.PasswordHash), []byte(hashPassword(site.PasswordSalt, password)))
}
func (a *App) accessCookie(site Site) string {
	h := hmac.New(sha256.New, []byte(a.cfg.Token))
	h.Write([]byte(site.ID + ":" + site.PasswordHash))
	return hex.EncodeToString(h.Sum(nil))
}
func (a *App) serve(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/s/")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	a.mu.RLock()
	var site *Site
	for i := range a.sites {
		if a.sites[i].ID == id {
			site = &a.sites[i]
			break
		}
	}
	a.mu.RUnlock()
	if site == nil {
		http.NotFound(w, r)
		return
	}
	if site.PasswordHash != "" {
		cookie, _ := r.Cookie("site_access_" + id)
		if cookie == nil || !hmac.Equal([]byte(cookie.Value), []byte(a.accessCookie(*site))) {
			if r.Method == http.MethodPost {
				r.ParseForm()
				if !a.passwordOK(*site, r.FormValue("password")) {
					http.Error(w, "invalid password", 403)
					return
				}
				http.SetCookie(w, &http.Cookie{Name: "site_access_" + id, Value: a.accessCookie(*site), Path: "/s/" + id, HttpOnly: true, SameSite: http.SameSiteLaxMode})
				http.Redirect(w, r, "/s/"+id, http.StatusSeeOther)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, "<!doctype html><meta charset=utf-8><title>需要访问密码</title><form method=post><label>请输入四位访问密码 <input name=password type=password inputmode=numeric pattern=\\d{4} maxlength=4 required></label><button>访问</button></form>")
			return
		}
	}
	http.ServeFile(w, r, filepath.Join(a.cfg.DataDir, "html", id+".html"))
}

var page = template.Must(template.ParseFiles("web/index.html"))

func (a *App) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	page.Execute(w, struct {
		Domain, Title string
		Count         int
	}{a.cfg.Domain, a.cfg.HomeTitle, len(a.sites)})
}
func logging(n http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		n.ServeHTTP(w, r)
	})
}
