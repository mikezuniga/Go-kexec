package main

import (
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"gopkg.in/ldap.v2"
)

var (
	MessageCreateFunctionFailed = "Failed to create function"
	MessageCallFunctionFailed   = "Failed to call function"
	MessageInternalServerError  = "Server Error"

	LoginTemplate      = template.Must(template.ParseFiles("html/login.html"))
	DashboardTemplate  = template.Must(template.ParseFiles("html/dashboard.html"))
	ConfFuncTemplate   = template.Must(template.ParseFiles("html/configure_func.html"))
	FuncCalledTemplate = template.Must(template.ParseFiles("html/func_called.html"))
	ErrorTemplate      = template.Must(template.ParseFiles("html/error.html"))
	DeleteFuncTemplate = template.Must(template.ParseFiles("html/func_deleted.html"))
)

func IndexPageHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	userName := getUserName(a, request)
	if userName != "" {
		//Already logged in, show dashboard
		//TODO: redirect or call the handler directly
		return DashboardHandler(a, response, request)
	} else {
		LoginTemplate.Execute(response, nil)
	}
	return nil
}

func LoginHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	name := request.FormValue("name")
	pass := request.FormValue("password")
	redirectTarget := "/"
	if name != "" && pass != "" {
		// ... check credentials
		ok, err := checkCredentials(a, name, pass)
		if !ok {
			errMsg := err.Error()
			// Check if it is a LDAP specific error
			for code, msg := range ldap.LDAPResultCodeMap {
				if ldap.IsErrorWithCode(err, code) {
					errMsg = msg
					break
				}
			}
			LoginTemplate.Execute(response, &LoginPage{LoginErr: true, ErrMsg: errMsg})
			return nil
		}

		// Put authenticated user into DB
		insertId, rowCnt, err := putUserIfNotExistedInDB(a, "", name)

		// Return internal server error if DB operation failed
		if err != nil {
			return StatusError{Code: http.StatusInternalServerError,
				Err: err, UserMsg: MessageInternalServerError}
		}

		if rowCnt > 0 {
			log.Printf("Successfully put user into DB, uid = %d", insertId)
		} else {
			log.Printf("User %s already in DB.", name)
		}

		setSession(a, name, response)
		redirectTarget = "/dashboard"
	}
	http.Redirect(response, request, redirectTarget, http.StatusFound)
	return nil
}

func LogoutHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	clearSession(response)
	log.Println("Logged out")
	http.Redirect(response, request, "/", http.StatusFound)
	return nil
}

func DashboardHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	userName := getUserName(a, request)
	if userName != "" {
		functions, err := getUserFunctions(a, userName, -1)
		// Cannot list the function, return a page with no function name
		if err != nil {
			log.Println("Cannot list functions for", userName)
			DashboardTemplate.Execute(response, &DashboardPage{Username: userName})
			return nil
		}

		DashboardTemplate.Execute(response, &DashboardPage{Username: userName, Functions: functions})
	} else {
		http.Redirect(response, request, "/", http.StatusFound)
	}
	return nil
}

func CreateFuncPageHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	userName := getUserName(a, request)
	if userName == "" {
		http.Redirect(response, request, "/", http.StatusFound)
	} else {
		ConfFuncTemplate.Execute(response, nil)
	}
	return nil
}

func ViewFuncPageHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	userName := getUserName(a, request)
	if userName == "" {
		http.Redirect(response, request, "/", http.StatusFound)
	} else {
		vars := mux.Vars(request)
		functionName := vars["function"]

		content, err := a.dal.GetFunction(userName, functionName)
		if err != nil {
			log.Println("Cannot get function", functionName)
			return StatusError{Code: http.StatusInternalServerError,
				Err: err, UserMsg: MessageInternalServerError}
		}
		ConfFuncTemplate.Execute(response, &ConfigFuncPage{
			EnableFuncName: false,
			FuncName:       functionName,
			FuncRuntime:    "python27",
			FuncContent:    content})
	}
	return nil
}

func DeleteFunctionHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	userName := getUserName(a, request)
	if userName == "" {
		http.Redirect(response, request, "/", http.StatusFound)
	} else {
		vars := mux.Vars(request)
		functionName := vars["function"]

		if err := a.dal.DeleteFunction(userName, functionName); err != nil {
			return StatusError{Code: http.StatusInternalServerError,
				Err: err, UserMsg: MessageInternalServerError}
		}
		DeleteFuncTemplate.Execute(response, nil)
	}
	return nil
}

func CreateFunctionHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	userName := getUserName(a, request)
	if userName == "" {
		// Empty username is not allowed to create function
		http.Redirect(response, request, "/", http.StatusFound)
	} else {

		// Read function code from the form
		// Before the function can be created, several steps needs to be
		// executed.
		//   2. Create the execution file for the function
		//   3. Write the function code to the execution file
		//   4. Build the function (ie build docker image)
		functionName := request.FormValue("functionName")
		runtime := request.FormValue("runtime")
		code := request.FormValue("codeTextarea")

		// Check if function already exists
		if _, err := a.dal.GetFunction(userName, functionName); err != sql.ErrNoRows {
			return StatusError{Code: http.StatusFound,
				Err:         errors.New(fmt.Sprintf("Function %s already exists for user %s.", functionName, userName)),
				UserMsg:     MessageCreateFunctionFailed,
				SendErrResp: true}

		}

		if err := createFunction(a, userName, functionName, runtime, code); err != nil {
			return StatusError{Code: http.StatusFound,
				Err:         err,
				UserMsg:     MessageCreateFunctionFailed,
				SendErrResp: true}
		}
	}
	return nil
}

func EditFunctionHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	userName := getUserName(a, request)
	if userName == "" {
		http.Redirect(response, request, "/", http.StatusFound)
	} else {
		functionName := request.FormValue("functionName")
		runtime := request.FormValue("runtime")
		code := request.FormValue("codeTextarea")

		if err := createFunction(a, userName, functionName, runtime, code); err != nil {
			return StatusError{Code: http.StatusFound,
				Err:         err,
				UserMsg:     MessageCreateFunctionFailed,
				SendErrResp: true}
		}
	}
	return nil
}

func CallHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	userName := getUserName(a, request)
	vars := mux.Vars(request)
	functionName := vars["function"]
	params := request.FormValue("params")

	if userName == "" {
		// Empty username is not allowed to call function
		http.Redirect(response, request, "/", http.StatusFound)
	} else {
		if functionName == "" {
			return StatusError{Code: http.StatusFound, Err: errors.New("Empty function name"),
				UserMsg: MessageCallFunctionFailed}
		}

		if params == "" {
			log.Println("Calling function", functionName)
		} else {
			log.Println("Calling function", functionName, "with parameters", params)
		}

		status, funcLog, err := callFunction(a, userName, functionName, params)
		if err != nil {
			return StatusError{Code: http.StatusFound, Err: err, UserMsg: MessageCallFunctionFailed}
		}

		FuncCalledTemplate.Execute(response, &CallResult{Result: status, Log: funcLog})
	}
	return nil
}
