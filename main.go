package main

import (
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/flowdev/gwiki/parser"
)

const (
	Suffix      = ".md"
	ContentDir  = "./content/"
	TemplateDir = "./tmpl/"
	Address     = ":1515"
	DateFormat  = "2006-01-02"
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
func (p *Page) Date() string {
	d := time.Now()
	if v, ok := p.FrontMatter["date"]; ok {
		if t, ok := v.(time.Time); ok {
			d = t
		} else {
			log.Printf("ERROR: Ill formatted date on page '%s': %#v", p.Path, v)
		}
	} else {
		log.Printf("WARNING: No date on page '%s'.", p.Path)
	}
	return d.Format(DateFormat)
}
func (p *Page) SetDate(d string) {
	t, err := time.Parse(DateFormat, d)
	if err != nil {
		log.Printf("ERROR: Ill formatted date for page '%s': %s", p.Path, d)
	} else {
		p.FrontMatter["date"] = t
	}
}
func (p *Page) Tags() []string {
	if v, ok := p.FrontMatter["tags"]; ok {
		if s, ok := v.([]string); ok {
			return s
		} else if es, ok := v.([]interface{}); ok {
			ss := make([]string, len(es))
			for i, e := range es {
				ss[i] = fmt.Sprintf("%s", e)
			}
			return ss
		} else {
			return []string{fmt.Sprintf("No_string_slice:%#v", v)}
		}
	} else {
		return nil
	}
}
func (p *Page) SetTags(t string) {
	ts := strings.Fields(t)
	if p.Mark == '+' {
		p.FrontMatter["tags"] = toInterSlice(ts)
	} else {
		p.FrontMatter["tags"] = ts
	}
}
func (p *Page) Language() string {
	return getString(p, "language")
}
func (p *Page) SetLanguage(l string) {
	p.FrontMatter["language"] = l
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
func (p *Page) SetDraft(d string) {
	p.FrontMatter["draft"] = strings.EqualFold(d, "true")
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
func toInterSlice(ss []string) []interface{} {
	is := make([]interface{}, len(ss))
	for i, s := range ss {
		is[i] = s
	}
	return is
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
	p := &Page{Path: path, Mark: mark(pg.FrontMatter()), Body: pg.Content()}
	md, err := pg.Metadata()
	if err != nil {
		return nil, fmt.Errorf("error parsing frontmatter of file '%s': %s", path, err)
	}
	if md == nil {
		return nil, fmt.Errorf("no frontmatter in file '%s': %s", path, err)
	}
	m := md.(map[string]interface{})
	p.FrontMatter = m

	return p, nil
}
func mark(fm []byte) rune {
	if len(fm) > 0 {
		return rune(fm[0])
	}
	return '+'
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
		log.Printf("ERROR: While loading the page '%s': %s\n", path, err)
		p = &Page{Path: path}
	}
	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, path string) {
	p, err := LoadPage(path)
	if err != nil {
		log.Printf("ERROR: Unable to load page '%s': %s\n", path, err)
		p = &Page{Path: path}
	}
	p.Body = []byte(r.FormValue("body"))
	p.SetDraft(r.FormValue("draft"))
	p.SetLanguage(r.FormValue("language"))
	p.SetDate(r.FormValue("date"))
	p.SetTitle(r.FormValue("title"))
	p.SetTags(r.FormValue("tags"))
	p.SetDescription(r.FormValue("description"))
	log.Printf("DEBUG: 'Saving' (draft: %t, lang: %s, date: %v, title: %s, tags: %v, desc: %s) body: %s\n",
		p.FrontMatter["draft"], p.FrontMatter["language"], p.FrontMatter["date"], p.FrontMatter["title"], p.FrontMatter["tags"], p.FrontMatter["description"], p.Body)
	err = p.Save()
	if err != nil {
		log.Printf("ERROR: %s\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	//http.Redirect(w, r, "/view/"+path, http.StatusFound)
	http.Redirect(w, r, "/edit/"+path, http.StatusFound)
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
	log.Printf("INFO: Starting web server on address: '%s'\n", Address)
	http.ListenAndServe(Address, nil)
}
