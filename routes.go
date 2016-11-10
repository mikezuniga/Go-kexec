package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

type Route struct {
	Name    string
	Method  string
	Pattern string
	Handler appRouteHandler
}

type Routes []Route

func (ah appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := ah.H(ah.appContext, w, r)
	if err != nil {
		switch e := err.(type) {
		case Error:
			// We can retrieve the status here and write out a specific
			// HTTP status code.
			log.Printf("HTTP %d - %s", e.Status(), e)
			errMsg := fmt.Sprintf("%s: %s", e.Message(), e)
			if e.SendErrorResponse() {
				http.Error(w, errMsg, e.Status())
			} else {
				ErrorTemplate.Execute(w, &ErrorPage{Message: errMsg})
			}
		default:
			// Any error types we don't specifically look out for default
			// to serving a HTTP 500
			http.Error(w, http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError)
		}
	}
}

func NewRouter(context *appContext) *mux.Router {

	router := mux.NewRouter()
	for _, route := range routes {
		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(appHandler{context, route.Handler})
	}

	router.PathPrefix("/").Handler(http.FileServer(http.Dir(context.conf.FileServerDir)))
	return router
}

var routes = Routes{
	Route{
		"Index",
		"GET",
		"/",
		IndexPageHandler,
	},
	Route{
		"Login",
		"POST",
		"/login",
		LoginHandler,
	},
	Route{
		"Logout",
		"GET",
		"/logout",
		LogoutHandler,
	},
	Route{
		"Dashboard",
		"GET",
		"/dashboard",
		DashboardHandler,
	},
	Route{
		"Create",
		"GET",
		"/create",
		CreateFuncPageHandler,
	},
	Route{
		"Create",
		"POST",
		"/create",
		CreateFunctionHandler,
	},
	Route{
		"View",
		"GET",
		"/functions/{function}",
		ViewFuncPageHandler,
	},
	Route{
		"Edit",
		"POST",
		"/functions/{function}/edit",
		EditFunctionHandler,
	},
	Route{
		"Delete",
		"POST",
		"/functions/{function}/delete",
		DeleteFunctionHandler,
	},
	Route{
		"Call",
		"POST",
		"/functions/{function}/call",
		CallHandler,
	},
	Route{
		"Call",
		"POST",
		"/users/{username}/functions/{function}/call",
		ApiCallFunctionHandler,
	},
}
