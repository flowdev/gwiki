package main

import (
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/flowdev/gwiki/parser"
)

const (
	Suffix      = ".md"
	ContentDir  = "./content/"
	TemplateDir = "./tmpl/"
)

var templates = template.Must(template.ParseFiles(TemplateDir+"edit.html", TemplateDir+"view.html"))
var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9/_-]+)$")

type Page struct {
	Path        string                 // from the URL and hints to the file
	FrontMatter map[string]interface{} // all FrontMatter params
	Mark        rune                   // mark for front matter format (YAML(-), TOML(+) or JSON({))
	Body        []byte                 // the content
}

func (p *Page) Title() string {
	return getString(p, "title")
}
func (p *Page) SetTitle(t string) {
	p.FrontMatter["title"] = t
}
func (p *Page) Description() string {
	return getString(p, "description")
}
func (p *Page) SetDescription(d string) {
	p.FrontMatter["description"] = d
}
func (p *Page) Date() time.Time {
	if v, ok := p.FrontMatter["date"]; ok {
		if t, ok := v.(time.Time); ok {
			return t
		} else {
			log.Printf("ERROR: Ill formatted date on page '%s': %#v", p.Path, v)
			var t time.Time
			return t // return zero value
		}
	} else {
		log.Printf("WARNING: No date on page '%s'.", p.Path)
		var t time.Time
		return t // return zero value
	}
}
func (p *Page) SetDate(d time.Time) {
	p.FrontMatter["date"] = d
}
func (p *Page) Tags() []string {
	if v, ok := p.FrontMatter["tags"]; ok {
		if s, ok := v.([]string); ok {
			return s
		} else {
			return []string{fmt.Sprintf("No_string_slice:%#v", v)}
		}
	} else {
		return nil
	}
}
func (p *Page) SetTags(t []string) {
	p.FrontMatter["tags"] = t
}
func (p *Page) Language() string {
	return getString(p, "language")
}
func (p *Page) SetLanguage(t string) {
	p.FrontMatter["language"] = t
}
func (p *Page) Draft() bool {
	if v, ok := p.FrontMatter["draft"]; ok {
		if b, ok := v.(bool); ok {
			return b
		} else {
			log.Printf("WARNING: Ill formated draft status '%#v' for page '%s'\n", v, p.Path)
			return true
		}
	} else {
		log.Printf("WARNING: Missing draft status. Default is 'true' for page '%s'\n", p.Path)
		return true
	}
}
func (p *Page) SetDraft(d bool) {
	p.FrontMatter["draft"] = d
}
func getString(p *Page, key string) string {
	if v, ok := p.FrontMatter[key]; ok {
		if s, ok := v.(string); ok {
			return s
		} else {
			return fmt.Sprintf("no string: %#v", v)
		}
	} else {
		return ""
	}
}

func (p *Page) Save() error {
	filename := ContentDir + p.Path + Suffix
	fout, err := os.Create(filename)
	if err != nil {
		return errors.New(fmt.Sprintf("unable to open or create page '%s': %s", filename, err))
	}
	fmBytes, err := parser.InterfaceToFrontMatter(p.FrontMatter, p.Mark)
	if err != nil {
		return errors.New(fmt.Sprintf("unable to generate front matter for page '%s': %s", filename, err))
	}
	_, err = fout.Write(fmBytes)
	if err != nil {
		return errors.New(fmt.Sprintf("unable to write front matter for page '%s': %s", filename, err))
	}
	_, err = fout.Write(p.Body)
	if err != nil {
		return errors.New(fmt.Sprintf("unable to write content for page '%s': %s", filename, err))
	}
	return nil
}

func LoadPage(path string) (*Page, error) {
	filename := ContentDir + path + Suffix
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	pg, err := parser.ReadFrom(f)
	if err != nil {
		return nil, err
	}
	p := &Page{Path: path, Mark: rune(pg.FrontMatter()[0]), Body: pg.Content()}
	md, err := pg.Metadata()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("error parsing frontmatter of file '%s': %s", path, err))
	}
	if md == nil {
		return nil, errors.New(fmt.Sprintf("no frontmatter in file '%s': %s", path, err))
	}
	m := md.(map[string]interface{})
	p.FrontMatter = m

	return p, nil
}

func viewHandler(w http.ResponseWriter, r *http.Request, path string) {
	p, err := LoadPage(path)
	if err != nil {
		log.Printf("ERROR: %s\n", err)
		http.Redirect(w, r, "/edit/"+path, http.StatusFound)
		return
	}
	renderTemplate(w, "view", p)
}

func editHandler(w http.ResponseWriter, r *http.Request, path string) {
	p, err := LoadPage(path)
	if err != nil {
		log.Printf("ERROR: %s\n", err)
		p = &Page{Path: path}
	}
	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, path string) {
	body := r.FormValue("body")
	p, err := LoadPage(path)
	if err != nil {
		log.Printf("ERROR: Creating page because of: %s\n", err)
		p = &Page{Path: path, Body: []byte(body)}
	}
	err = p.Save()
	if err != nil {
		log.Printf("ERROR: %s\n", err)
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
		log.Printf("ERROR: %s\n", err)
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
