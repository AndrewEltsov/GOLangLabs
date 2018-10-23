package main

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/gocraft/web"
)

var templates = template.Must(template.ParseGlob("templates/*.html"))

//Context is temporary empty structure
type Context struct {
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

func main() {
	rootRouter := web.New(Context{}). // Create your router
						Middleware(web.LoggerMiddleware).     // Use some included middleware
						Middleware(web.ShowErrorsMiddleware). // ...
						Get("/", (*Context).renderHomePage).  // Add a route
						Get("/register", (*Context).renderRegistrationPage).
						Post("/register", (*Context).performRegistration)

	http.ListenAndServe("localhost:8080", rootRouter) // Start the server!
}
