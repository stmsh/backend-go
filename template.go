package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"time"
)

func createTemplate(paths []string) *template.Template {
	tmpl := template.New("").Funcs(template.FuncMap{
		"format_duration": func(t time.Duration) string {
			return fmt.Sprintf("%d:%02d", int(t.Minutes()), int(t.Seconds())%60)
		}})

	for _, path := range paths {
		tmpl = template.Must(tmpl.ParseGlob(path))
	}

	return tmpl
}

var Template = createTemplate([]string{"templates/*.html", "templates/**/*.html"})

func Render(view string, data any) []byte {
	buff := bytes.Buffer{}
	err := Template.ExecuteTemplate(&buff, view, data)
	if err != nil {
		log.Print(err)
	}

	return buff.Bytes()
}
