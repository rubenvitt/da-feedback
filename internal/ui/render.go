package ui

import (
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
)

type Renderer struct {
	templates map[string]*template.Template
}

func NewRenderer(templateFS fs.FS) (*Renderer, error) {
	r := &Renderer{templates: make(map[string]*template.Template)}

	funcs := template.FuncMap{
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i + 1
			}
			return s
		},
		"deref": func(p *string) string {
			if p == nil {
				return ""
			}
			return *p
		},
		"derefInt": func(p *int) int {
			if p == nil {
				return 0
			}
			return *p
		},
	}

	base := "base.html"

	pages, err := fs.Glob(templateFS, "*/*.html")
	if err != nil {
		return nil, err
	}

	for _, page := range pages {
		name := filepath.ToSlash(page)
		t, err := template.New("").Funcs(funcs).ParseFS(templateFS, base, page)
		if err != nil {
			return nil, err
		}
		r.templates[name] = t
	}

	return r, nil
}

func (rn *Renderer) Render(w http.ResponseWriter, name string, status int, data any) {
	t, ok := rn.templates[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	t.ExecuteTemplate(w, "base", data)
}
