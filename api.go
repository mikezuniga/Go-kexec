package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func ApiCallFunctionHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	vars := mux.Vars(request)
	userName := vars["username"]
	functionName := vars["function"]

	// Get function parameters from request body
	params, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return StatusError{Code: http.StatusFound, Err: err, UserMsg: MessageCallFunctionFailed}
	}
	paramsStr := string(params)
	if paramsStr == "" {
		log.Println("Calling function", functionName)
	} else {
		log.Println("Calling function", functionName, "with parameters", paramsStr)
	}

	// Call function. This will create a job in OpenShift
	status, funcLog, err := callFunction(a, userName, functionName, paramsStr)
	if err != nil {
		return StatusError{Code: http.StatusFound, Err: err, UserMsg: MessageCallFunctionFailed}
	}
	// Write to response
	fmt.Fprintf(response, "Execution %s.\nResult:\n%s\n", status, string(funcLog))
	return nil
}
