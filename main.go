package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/gocraft/web"
	_ "github.com/lib/pq"
)

var templates = template.Must(template.ParseGlob("templates/*.html"))

var cache = make(map[int]Document)

//Context is temporary empty structure
type Context struct {
	dbPointer *sql.DB
}

type Document struct {
	Name string
	Data []byte
}

func (c *Context) renderHomePage(rw web.ResponseWriter, req *web.Request) {
	err := templates.ExecuteTemplate(rw, "home.html", c)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}
}

func (c *Context) renderRegistrationPage(rw web.ResponseWriter, req *web.Request) {
	err := templates.ExecuteTemplate(rw, "registration.html", c)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}
}

func (c *Context) performRegistration(rw web.ResponseWriter, req *web.Request) {
	req.ParseForm()
	for key, value := range req.Form {
		str := fmt.Sprintf("%s: %s\n", key, value[0])
		fmt.Fprint(rw, str)
	}
}

func (c *Context) getDocList(rw web.ResponseWriter, req *web.Request) {

	rows, err := c.dbPointer.Query("SELECT id, name FROM docs;")
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}
	defer rows.Close()

	names := make(map[int]string)
	for i := 0; rows.Next(); i++ {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
		}
		names[id] = name
	}

	fmt.Println(names)

	err = templates.ExecuteTemplate(rw, "list.html", names)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}
}

func (c *Context) getDoc(rw web.ResponseWriter, req *web.Request) {
	id, err := strconv.Atoi(req.PathParams["id"])
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	type docContent struct {
		Title string
		Body  string
	}
	var doc docContent

	value, isCached := cache[id]

	if isCached {
		doc = docContent{value.Name,
			string(value.Data)}

	} else {
		rows, err := c.dbPointer.Query(fmt.Sprintf("SELECT name, data FROM docs WHERE id=%d;", id))
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
		}
		defer rows.Close()

		if rows.Next() {
			var name string
			var data []byte
			if err := rows.Scan(&name, &data); err != nil {
				http.Error(rw, err.Error(), http.StatusInternalServerError)
			}
			doc = docContent{name,
				string(data)}
			cache[id] = Document{Name: name,
				Data: data}
		} else {
			doc = docContent{"Такого документа не существует",
				"Извините :("}
		}

	}
	err = templates.ExecuteTemplate(rw, "doc.html", doc)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}
}

func (c *Context) sendDocForm(rw web.ResponseWriter, req *web.Request) {
	err := templates.ExecuteTemplate(rw, "send.html", c)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}
}

func (c *Context) sendDoc(rw web.ResponseWriter, req *web.Request) {
	file, _, err := req.FormFile("file_data")
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	data, err := ioutil.ReadAll(file)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	_, err = c.dbPointer.Exec(fmt.Sprintf("INSERT INTO docs(name, data) VALUES('%s', '\\x%x');", req.FormValue("file_name"), data))
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	http.Redirect(rw, req.Request, "/docs", http.StatusTemporaryRedirect)

}

func main() {
	connStr := "user=postgres_user password=pass dbname=docs_list sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		fmt.Println(err)
	}
	//defer db.Close()

	c := Context{
		dbPointer: db}
	rootRouter := web.New(c). // Create your router
					Middleware(web.LoggerMiddleware).     // Use some included middleware
					Middleware(web.ShowErrorsMiddleware). // ...
					Get("/", (*Context).renderHomePage).  // Add a route
		/* Get("/register", (*Context).renderRegistrationPage).
		Post("/register", (*Context).performRegistration) */
		Get("/docs", c.getDocList).
		Get("/docs/:id", c.getDoc).
		Get("/send", c.sendDocForm).
		Post("/send", c.sendDoc)

	http.ListenAndServe("localhost:8080", rootRouter) // Start the server!
}
