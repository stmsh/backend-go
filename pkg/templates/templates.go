package templates

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"log"
	"time"
)

//go:embed *.html */*.html
var templates embed.FS

func formatDuration(t time.Duration) string {
	return fmt.Sprintf("%d:%02d", int(t.Minutes()), int(t.Seconds())%60)
}

var t = template.Must(
	template.
		New("").
		Funcs(template.FuncMap{"format_duration": formatDuration}).
		ParseFS(templates, "*.html", "*/*.html"),
)

func Render(view string, data any) []byte {
	buff := bytes.Buffer{}
	err := t.ExecuteTemplate(&buff, view, data)
	if err != nil {
		log.Print(err)
	}

	return buff.Bytes()
}
