package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Config struct{ Domain, Token, DataDir, Port string }
type Site struct{ ID, Title, Summary, CreatedAt string }
type App struct {
	cfg   Config
	mu    sync.RWMutex
	sites []Site
}

func main() {
	c := Config{env("DOMAIN", "site.tsio.top"), os.Getenv("UPLOAD_TOKEN"), env("DATA_DIR", "/data"), env("PORT", "18080")}
	if c.Token == "" {
		log.Fatal("UPLOAD_TOKEN is required")
	}
	a := &App{cfg: c}
	if e := a.load(); e != nil {
		log.Fatal(e)
	}
	m := http.NewServeMux()
	m.HandleFunc("/api/auth/check", a.authCheck)
	m.HandleFunc("/api/sites", a.api)
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
func (a *App) api(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		a.mu.RLock()
		defer a.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(a.sites)
		return
	}
	if r.Method != http.MethodPost || strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") != a.cfg.Token {
		http.Error(w, "unauthorized", 401)
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
	a.mu.Lock()
	a.sites = append([]Site{{id, title, summary, time.Now().UTC().Format(time.RFC3339)}}, a.sites...)
	e = a.save()
	a.mu.Unlock()
	if e != nil {
		os.Remove(p)
		http.Error(w, "storage error", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id, "url": "https://" + a.cfg.Domain + "/s/" + id})
}
func (a *App) serve(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/s/")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
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
		Domain string
		Sites  []Site
	}{a.cfg.Domain, a.sites})
}
func logging(n http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		n.ServeHTTP(w, r)
	})
}
