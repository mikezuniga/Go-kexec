package dal

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"
)

var funcContentTemp = `
def foo():
	print("Testing DAL (#%d).")
foo()
`

var (
	testUsername = "TestUser"
	db           *MySQL
	userId       int64
	functionId   int64
	params       = "{\"x\":1}"
	status       = "Failed"
	uuid         = "xxx"
	execLog      = "log"
)

func TestMain(m *testing.M) {
	var err error
	config := &DalConfig{
		DBHost:   "100.73.145.91",
		Username: "kexec",
		Password: "password",

		DBName: "kexectest",

		UsersTable:      "users",
		FunctionsTable:  "functions",
		ExecutionsTable: "executions",
	}

	db, err = NewMySQL(config)

	if err != nil {
		panic(err)
	}

	// Clear DB before test
	if err = db.ClearDatabase(); err != nil {
		panic(err)
	}

	code := m.Run()

	// Clear DB after test
	if err = db.ClearDatabase(); err != nil {
		panic(err)
	}

	os.Exit(code)
}

func TestPutUserIfNotExisted(t *testing.T) {
	if db == nil {
		return
	}
	lastId, rowCount, err := db.PutUserIfNotExisted("", testUsername)
	userId = lastId
	if err != nil {
		t.Error(err)
	}
	if rowCount != 1 {
		t.Error("First user insert error")
	}
	lastId, rowCount, err = db.PutUserIfNotExisted("", testUsername)
	if rowCount != 0 {
		t.Error("Second user insert error")
	}
}

func TestPutFunction(t *testing.T) {
	funcList := make([]*Function, 0, 5)
	for i := 0; i < 3; i++ {
		function := &Function{
			UserID:  userId,
			Name:    fmt.Sprintf("TestFunction%d", i+1),
			Content: fmt.Sprintf(funcContentTemp, i+1),
		}
		funcList = append(funcList, function)
	}

	for i, function := range funcList {
		log.Printf("Inserting function %s...", function.Name)
		_, rowCount, err := db.PutFunction("", function.Name, function.Content, function.UserID)
		if err != nil {
			t.Error(err)
		}

		if rowCount != 1 {
			t.Error("Function " + string(i) + " insert error")
		}
	}
	// Insert again. No rows should be updated.
	for i, function := range funcList {
		log.Printf("Inserting function %s...", function.Name)
		_, rowCount, err := db.PutFunction("", function.Name, function.Content, function.UserID)
		if err != nil {
			t.Error(err)
		}

		if rowCount != 0 {
			t.Error("Function " + string(i) + " insert error")
		}
	}

}

func TestUpdateFunction(t *testing.T) {
	funcList := make([]*Function, 0, 5)
	for i := 0; i < 3; i++ {
		function := &Function{
			UserID:  userId,
			Name:    fmt.Sprintf("TestFunction%d", i+1),
			Content: fmt.Sprintf(funcContentTemp, i+2),
		}
		funcList = append(funcList, function)
	}

	for i, function := range funcList {
		log.Printf("Inserting function %s...", function.Name)
		_, rowCount, err := db.PutFunction("", function.Name, function.Content, function.UserID)
		if err != nil {
			t.Error(err)
		}

		if rowCount != 1 {
			t.Error("Function " + string(i) + " insert error")
		}
	}
}

func TestListFunctionsOfUser(t *testing.T) {
	functions, err := db.ListFunctionsOfUser(testUsername, -1)
	if err != nil {
		t.Error(err)
	}
	if len(functions) != 3 {
		t.Error("Size of function list is not right.")
	}
}

func TestGetFunction(t *testing.T) {
	function, err := db.GetFunction(testUsername, "TestFunction1")
	if err != nil {
		t.Error(err)
	}
	if function.UserID != userId ||
		function.Name != "TestFunction1" ||
		function.Content != fmt.Sprintf(funcContentTemp, 2) {
		t.Error("Get function error")
	}
	// Function id of TestFunction1
	functionId = function.ID
}

func TestPutExecution(t *testing.T) {
	_, rowCount, err := db.PutExecution(functionId, params, status, uuid, execLog, time.Now())
	if err != nil {
		t.Error(err)
	}
	if rowCount != 1 {
		t.Error("Put execution error")
	}
}

func TestListExecution(t *testing.T) {
	exec, err := db.ListExecution(testUsername, "TestFunction1")
	if err != nil {
		t.Error(err)
	}
	if exec[0].FunctionID != functionId ||
		exec[0].Params != params ||
		exec[0].Status != status ||
		exec[0].Uuid != uuid ||
		exec[0].Log != execLog {
		t.Error("List execution error")
	}
}

func TestDeleteFunction(t *testing.T) {
	// Delete TestFunction1
	err := db.DeleteFunction(testUsername, "TestFunction1")
	if err != nil {
		t.Error(err)
	}
	functions, err := db.ListFunctionsOfUser(testUsername, -1)
	if err != nil {
		t.Error(err)
	}
	if len(functions) != 2 {
		t.Error("Size of function list is not right.")
	}
}
