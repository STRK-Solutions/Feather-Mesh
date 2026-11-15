package gateway

import (
	"embed"
	"html/template"

	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/control"
	"github.com/STRK-Solutions/Feather-Mesh/web_demo/internal/pipeline"
)

type page struct {
	Account              control.Account
	Accounts             []control.Account
	Workspace            *control.Workspace
	Workspaces           []workspacePage
	Grants               []control.Grant
	LifecycleState       string
	WorkspaceUpdating    bool
	WarningAt            string
	RetentionNotified    string
	RetentionDeleteAfter string
	CSRF                 string
	Config               Config
	Policy               control.Policy
	Jobs                 []control.Job
	PipelineJobs         []pipeline.Status
	PipelineAvailable    bool
	BrokerStatus         BrokerStatus
}

//go:embed templates/instrument.css templates/home.html templates/review.html templates/pipeline.html
var pageTemplates embed.FS

var pageTemplateFunctions = template.FuncMap{
	"instrumentCSS": func() template.CSS {
		content, err := pageTemplates.ReadFile("templates/instrument.css")
		if err != nil {
			panic(err)
		}
		return template.CSS(content)
	},
}

var home = template.Must(template.New("home.html").Funcs(pageTemplateFunctions).ParseFS(pageTemplates, "templates/home.html"))
var review = template.Must(template.New("review.html").Funcs(pageTemplateFunctions).ParseFS(pageTemplates, "templates/review.html"))
var pipelineTemplateV2 = template.Must(template.New("pipeline.html").Funcs(pageTemplateFunctions).ParseFS(pageTemplates, "templates/pipeline.html"))
