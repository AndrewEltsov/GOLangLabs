package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gocraft/web"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

var templates = template.Must(template.ParseGlob("templates/*.html"))

var mutex sync.RWMutex

var cache = make(map[int]*Document)

//Context
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
	rw.Header().Set("Location", "/docs")
	rw.WriteHeader(http.StatusTemporaryRedirect)
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
	for rows.Next() { //
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

	mutex.RLock()
	value, isCached := cache[id]
	mutex.RUnlock()

	if isCached {
		renderDoc(rw, value.Name, string(value.Data))
	} else {
		result := c.dbPointer.QueryRow("SELECT name, data FROM docs WHERE id=$1;", id)

		var name string
		var data []byte
		err = result.Scan(&name, &data)
		if err != nil {
			if err == sql.ErrNoRows {
				renderDoc(rw, "Такого документа не существует", "Извините :(")
			}
			http.Error(rw, err.Error(), http.StatusInternalServerError)
		}
		mutex.Lock()
		cache[id] = &Document{Name: name,
			Data: data}
		mutex.Unlock()

		renderDoc(rw, name, string(data))
	}
}

func (c *Context) deleteDoc(rw web.ResponseWriter, req *web.Request) {
	id, err := strconv.Atoi(req.PathParams["id"])
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	mutex.RLock()
	_, isCached := cache[id]
	mutex.RUnlock()

	if isCached {
		mutex.Lock()
		delete(cache, id)
		mutex.Unlock()
	}

	_, err = c.dbPointer.Exec("DELETE FROM docs WHERE id=$1;", id)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}
	http.Redirect(rw, req.Request, "/docs", http.StatusTemporaryRedirect)
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
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	_, err = c.dbPointer.Exec("INSERT INTO docs(name, data) VALUES($1, $2);", req.FormValue("file_name"), data)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	rw.Header().Set("Location", "/docs")
	rw.WriteHeader(http.StatusMovedPermanently)
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

	rw.Header().Set("Location", "/login")
	rw.WriteHeader(http.StatusTemporaryRedirect)
}

func (c *Context) signin(rw web.ResponseWriter, req *web.Request) {
	err := req.ParseForm()
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	result := c.dbPointer.QueryRow("select id, hashed_password from users where username=$1", req.PostFormValue("username"))

	var hash string
	var id int
	err = result.Scan(&id, &hash)
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

	token := uuid.New()
	log.Println(token.String())
	_, err = c.dbPointer.Exec("INSERT INTO sessions(token, user_id, time) VALUES($1, $2, $3);", token.String(), id, time.Now())
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	http.SetCookie(rw, &http.Cookie{Name: "token", Value: token.String()})
	rw.Header().Set("Location", "/docs")
	rw.WriteHeader(http.StatusFound)
}

func (c *Context) signout(rw web.ResponseWriter, req *web.Request) {
	token, _ := req.Cookie("token")
	http.SetCookie(rw, &http.Cookie{Name: "token", Value: ""})

	_, err := c.dbPointer.Exec("DELETE FROM sessions WHERE token=$1;", token.Value)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	rw.Header().Set("Location", "/login")
	rw.WriteHeader(http.StatusFound)
}

func (c *Context) checkAuth(rw web.ResponseWriter, r *web.Request, next web.NextMiddlewareFunc) {
	log.Println("checkAuth")
	log.Println(r.Request.URL.String())
	if r.RequestURI != "/login" && r.RequestURI != "/signin" && r.RequestURI != "/register" && r.RequestURI != "/signup" && r.RequestURI != "/favicon.ico" {
		log.Println("checkAuth for requests")
		var token, err = r.Request.Cookie("token")
		if err != nil || token.Value == "" {
			log.Println("token not found")
			rw.Header().Set("Location", "/login")
			rw.WriteHeader(http.StatusFound)
		} else {
			log.Println("token found")
			if c.getSession(token.Value) {
				next(rw, r)
			} else {
				http.SetCookie(rw, &http.Cookie{Name: "token", Value: ""})
				rw.Header().Set("Location", "/login")
				rw.WriteHeader(http.StatusFound)
			}
		}
	} else {
		next(rw, r)
	}
}

func (c *Context) getSession(token string) bool {
	log.Println(token)
	result := c.dbPointer.QueryRow("SELECT time FROM sessions WHERE token=$1;", token)

	var sessionTime time.Time

	err := result.Scan(&sessionTime)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("No session")
			return false
		}
		log.Println(err.Error())
		return false
	}

	if time.Since(sessionTime) > time.Hour {
		log.Println("Session outdated")
		_, err = c.dbPointer.Exec("DELETE FROM sessions WHERE token=$1;", token)
		return false
	}

	return true
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
					Middleware(c.checkAuth).
					Get("/", c.renderHomePage).
					Get("/login", c.renderLoginPage).
					Post("/signin", c.signin).
					Get("/docs", c.getDocList).
					Get("/docs/:id", c.getDoc).
					Get("/send", c.sendDocForm).
					Post("/send", c.sendDoc).
					Get("/delete/:id", c.deleteDoc).
					Post("/signout", c.signout).
					Get("/register", c.renderRegistrationPage).
					Post("/signup", c.signup)

	panic(http.ListenAndServe("localhost:8080", rootRouter)) // Start the server
}
