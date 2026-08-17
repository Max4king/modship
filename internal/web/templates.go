package web

import (
	"embed"
	"html/template"
	"io/fs"
)

//go:embed templates/*.html
var templateFS embed.FS

// templates is the parsed template set shared by all handlers.
var templates *template.Template

func init() {
	// Strip the "templates/" prefix so we can reference by base name.
	sub, err := fs.Sub(templateFS, "templates")
	if err != nil {
		panic("web: embed templates: " + err.Error())
	}
	templates = template.Must(template.New("").ParseFS(sub, "*.html"))
}
