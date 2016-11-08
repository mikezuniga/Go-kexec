package dal

import (
	"errors"
	"fmt"
	"log"
	"testing"
	"time"
)

var funcContentTemp = `
def foo():
	print("Testing DAL (#%d).")
foo()
`

func TestMain(m *testing.M) {

	config := &DalConfig{
		dbhost:   "100.73.145.91",
		username: "kexec",
		password: "password",

		dbname: "kexectest",

		usersTable:      "users",
		functionsTable:  "functions",
		executionsTable: "executions",
	}

	dal, err := NewMySQL(config)

	if err != nil {
		panic(err)
	}

	if err = dal.Ping(); err != nil {
		panic(err)
	}

	// Clear DB before test
	if err = dal.ClearDatabase(); err != nil {
		panic(err)
	}

	testUsername := "TestUser"

	log.Printf("Inserting user...")
	lastId, rowCount, err := dal.PutUserIfNotExisted("", testUsername)
	userId := lastId
	if err != nil {
		panic(err)
	}
	log.Printf("Last ID: %d, Rows affected: %d", lastId, rowCount)

	funcList := make([]*Function, 0, 5)
	for i := 0; i < 3; i++ {
		function := &Function{
			ID:      -1,
			UserID:  userId,
			Name:    fmt.Sprintf("TestFunction%d", i+1),
			Content: fmt.Sprintf(funcContentTemp, i+1),
			Created: time.Now(),
		}
		funcList = append(funcList, function)
	}

	for _, function := range funcList {
		log.Printf("Inserting function %s...", function.Name)
		lastId, rowCount, err = dal.PutFunctionIfNotExisted("", function.Name, function.Content, function.UserID)
		if err != nil {
			panic(err)
		}
		log.Printf("Last ID: %d, Rows affected: %d", lastId, rowCount)
	}

	functions, err := dal.ListFunctionsOfUser("default", testUsername, -1)
	if err != nil {
		panic(err)
	}
	if len(functions) != len(funcList) {
		panic(errors.New("Size of function list is not right."))
	}

	// Clear DB after test
	if err = dal.ClearDatabase(); err != nil {
		panic(err)
	}
}
