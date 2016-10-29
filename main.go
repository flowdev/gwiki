package main

import (
	"html/template"
	"io/ioutil"
	"net/http"
	"regexp"
	"time"
)

const (
	Suffix      = ".md"
	ContentDir  = "./content/"
	TemplateDir = "./tmpl/"
)

var templates = template.Must(template.ParseFiles(TemplateDir+"edit.html", TemplateDir+"view.html"))
var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9/_-]+)$")

type Page struct {
	Path        string    // from the URL and hints to the file
	Title       string    // for FrontMatter
	Description string    // for FrontMatter
	Tags        []string  // for FrontMatter
	Date        time.Time // for FrontMatter
	Body        []byte    // the content
}

func (p *Page) save() error {
	filename := ContentDir + p.Path + Suffix
	// TODO: write FontMatter!!!
	return ioutil.WriteFile(filename, p.Body, 0600)
}

func loadPage(path string) (*Page, error) {
	filename := ContentDir + path + Suffix
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	} else {
		return &Page{Path: path, Body: body}, nil
	}
}

func viewHandler(w http.ResponseWriter, r *http.Request, path string) {
	p, err := loadPage(path)
	if err != nil {
		http.Redirect(w, r, "/edit/"+path, http.StatusFound)
		return
	}
	renderTemplate(w, "view", p)
}

func editHandler(w http.ResponseWriter, r *http.Request, path string) {
	p, err := loadPage(path)
	if err != nil {
		p = &Page{Path: path}
	}
	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, path string) {
	body := r.FormValue("body")
	p := &Page{Path: path, Body: []byte(body)}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+path, http.StatusFound)
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2]) // The path is the second subexpression.
	}
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	http.ListenAndServe(":1515", nil)
}
