package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type ApiCallResult struct {
	Result  string `json:"result"`
	Log     string `json:"log"`
	Message string `json:"message"`
}

var ResError = "Error"

func ApiCallFunctionHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {

	res := callUserFunction(a, request)

	// Log the error if there is one
	if res.Message != "" {
		log.Println(res.Message)
	}

	// Write to response
	response.Header().Set("Content-Type", "application/json; charset=UTF-8")
	response.WriteHeader(http.StatusOK)
	e := json.NewEncoder(response)
	e.SetIndent("", "\t")
	if err := e.Encode(res); err != nil {
		return StatusError{http.StatusInternalServerError, err, MessageCallFunctionFailed, true}
	}
	return nil
}

func callUserFunction(a *appContext, request *http.Request) ApiCallResult {
	vars := mux.Vars(request)
	userName := vars["username"]
	functionName := vars["function"]
	// Sanity check
	if userName == "" || functionName == "" {
		return ApiCallResult{ResError, "", "Missing user name or function name."}
	}

	// Check if function already exists
	_, err := a.dal.GetFunction(userName, functionName)
	if err == sql.ErrNoRows {
		return ApiCallResult{ResError, "", fmt.Sprintf("Function %s not exist for user %s.", functionName, userName)}
	} else if err != nil {
		return ApiCallResult{ResError, "", err.Error()}
	}

	// Get function parameters from request body
	params, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return ApiCallResult{ResError, "", err.Error()}
	}
	paramsStr := string(params)
	if paramsStr == "" {
		log.Println("Calling function", functionName, "for user", userName)
	} else {
		log.Println("Calling function", functionName, "with parameters", paramsStr, "for user", userName)
	}

	// Call function. This will create a job in OpenShift
	timestamp := time.Now()
	res, err := callFunction(a, userName, functionName, paramsStr)
	if err != nil {
		return ApiCallResult{ResError, "", err.Error()}
	}

	// Insert function execution into DB
	if err := PutFunctionExecution(a, userName, functionName, paramsStr, res, timestamp); err != nil {
		return ApiCallResult{ResError, "", err.Error()}
	}

	return ApiCallResult{res.Result, res.Log, ""}
}
