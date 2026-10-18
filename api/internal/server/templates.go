package server

import (
	"embed"
	"html/template"
)

// templateFS embeds the HTML templates used by Server-level handlers (LTI
// error page, VNC launcher, multi-VM console). Templates are parsed once at
// package init; html/template auto-escapes user-supplied data for XSS safety.
//
//go:embed templates/*.gohtml
var templateFS embed.FS

var (
	ltiErrorTemplate    = template.Must(template.ParseFS(templateFS, "templates/lti_error.gohtml"))
	vncLauncherTemplate = template.Must(template.ParseFS(templateFS, "templates/vnc_launcher.gohtml"))
	ltiConsoleTemplate  = template.Must(template.ParseFS(templateFS, "templates/lti_console.gohtml"))
)
