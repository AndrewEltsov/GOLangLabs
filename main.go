package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"

	"github.com/gocraft/web"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

var templates = template.Must(template.ParseGlob("templates/*.html"))

var mutex = &sync.Mutex{}

var cache = make(map[int]Document)

//Context is temporary empty structure
type Context struct {
	dbPointer *sql.DB
}

type Document struct {
	Name string
	Data []byte
}

type User struct {
	Username       string `pq:"username"`
	HashedPassword string `pq:"hashed_password"`
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

func (c *Context) renderLoginPage(rw web.ResponseWriter, req *web.Request) {
	err := templates.ExecuteTemplate(rw, "login.html", c)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
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

	err = templates.ExecuteTemplate(rw, "list.html", names)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}
}

func renderDoc(rw web.ResponseWriter, title, data string) {
	err := templates.ExecuteTemplate(rw, "doc.html", struct {
		Title string
		Data  string
	}{
		title,
		data})
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}
}

func (c *Context) getDoc(rw web.ResponseWriter, req *web.Request) {
	id, err := strconv.Atoi(req.PathParams["id"])
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	mutex.Lock()
	value, isCached := cache[id]
	mutex.Unlock()

	if isCached {
		renderDoc(rw, value.Name, string(value.Data))
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

			mutex.Lock()
			cache[id] = Document{Name: name,
				Data: data}
			mutex.Unlock()

			renderDoc(rw, name, string(data))
		} else {
			renderDoc(rw, "Такого документа не существует", "Извините :(")
		}

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

	_, err = c.dbPointer.Exec("INSERT INTO docs(name, data) VALUES($1, $2);", req.FormValue("file_name"), data)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	http.Redirect(rw, req.Request, "/docs", http.StatusTemporaryRedirect)

}

func (c *Context) signup(rw web.ResponseWriter, req *web.Request) {
	err := req.ParseForm()
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.PostFormValue("password")), 8)

	_, err = c.dbPointer.Exec("insert into users(username, hashed_password) values($1, $2)", req.PostFormValue("username"), string(hashedPassword))
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}
}

func (c *Context) signin(rw web.ResponseWriter, req *web.Request) {
	err := req.ParseForm()
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	result := c.dbPointer.QueryRow("select hashed_password from users where username=$1", req.PostFormValue("username"))

	var hash string
	err = result.Scan(&hash)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(rw, err.Error(), http.StatusUnauthorized)
		}
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.PostFormValue("password")))
	if err != nil {
		http.Error(rw, err.Error(), http.StatusUnauthorized)
	}
}

func main() {
	connStr := "user=postgres_user password=pass dbname=docs_list sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		fmt.Println(err)
	}
	defer db.Close()

	c := Context{
		dbPointer: db}
	rootRouter := web.New(c). // Create your router
					Middleware(web.LoggerMiddleware).     // Use some included middleware
					Middleware(web.ShowErrorsMiddleware). // ...
					Get("/", (*Context).renderHomePage).  // Add a route
					Get("/docs", c.getDocList).
					Get("/docs/:id", c.getDoc).
					Get("/send", c.sendDocForm).
					Post("/send", c.sendDoc).
					Get("/register", c.renderRegistrationPage).
					Post("/signup", c.signup).
					Get("/login", c.renderLoginPage).
					Post("/signin", c.signin)

	http.ListenAndServe("localhost:8080", rootRouter) // Start the server!
}
