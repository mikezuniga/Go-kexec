package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Symantec/Go-kexec/docker"
	"github.com/wayn3h0/go-uuid"
	"gopkg.in/ldap.v2"
)

func createFunction(a *appContext, userName, functionName, runtime, code string) error {
	// Check if function name is empty;
	// check if runtime template is chosen;
	// check if the input code is empty.
	if functionName == "" {
		return errors.New("Function name is empty.")
	} else if runtime == "" {
		return errors.New("No runtime selected.")
	} else if code == "" {
		return errors.New("Function code is empty.")
	}

	newCode := formatCode(code, functionName)
	log.Printf("Code uploaded:\n%s", newCode)
	log.Printf("Start creating function \"%s\" with runtime \"%s\"", functionName, runtime)

	// Create a time based uuid as part of the context directory name
	uuid, err := uuid.NewTimeBased()

	if err != nil {
		log.Println("Failed to create uuid for function call.")
		return err
	}

	uuidStr := uuid.String()
	userCtx := userName + "-" + uuidStr

	// Create the execution file for the function
	ctxDir := filepath.Join(docker.IBContext, userCtx)

	if err := os.Mkdir(ctxDir, os.ModePerm); err != nil {
		return err
	}

	exeFileName := filepath.Join(ctxDir, docker.ExecutionFile)
	exeFile, err := os.Create(exeFileName)

	if err != nil {
		return err
	}
	defer exeFile.Close()

	// Write the function into the execution file
	if _, err = exeFile.WriteString(newCode); err != nil {
		return err
	}

	// Build funtion
	if err = a.d.BuildFunction(a.conf.DockerRegistry, userName, functionName, runtime, ctxDir); err != nil {
		log.Println("Build function failed")
		return err
	}

	// Register function to configured docker registry
	if err = docker.RegisterFunction(a.conf.DockerRegistry, userName, functionName); err != nil {
		log.Println("Register function failed")
		return err
	}

	// Put function into db
	if err = putUserFunction(a, userName, functionName, code, -1); err != nil {
		log.Println("Failed to put function into DB")
		return err
	}

	// If all the above operation succeeded, the function is created
	// successfully.
	return nil
}

//return success/failed, log and error
func callFunction(a *appContext, userName, functionName, params string) (string, string, error) {
	// create a uuid for each function call. This uuid can be
	// seen as the execution id for the function (notice there
	// are multiple executions for a single function)
	uuid, err := uuid.NewTimeBased()

	if err != nil {
		log.Println("Failed to create uuid for function call.")
		return "", "", err
	}

	uuidStr := uuid.String() // uuidStr needed when fetching log

	// Create a namespace for the user and run the job
	// in that namespace
	nsName := strings.Replace(userName, "_", "-", -1) + "-serverless"
	if _, err := a.k.CreateUserNamespaceIfNotExist(nsName); err != nil {
		log.Println("Failed to get/create user namespace", nsName)
		return "", "", err
	}
	jobName := functionName + "-" + uuidStr
	image := a.conf.DockerRegistry + "/" + userName + "/" + functionName
	labels := make(map[string]string)

	if err := a.k.CreateFunctionJob(jobName, image, params, nsName, labels); err != nil {
		log.Println("Failed to call function", functionName)
		return "", "", err
	}
	// Run the job
	status, err := a.k.RunJob(jobName, nsName)
	if err != nil {
		return "", "", err
	}

	// Get the log
	funcLog, err := a.k.GetFunctionLog(jobName, nsName)
	if err != nil {
		return "", "", err
	}
	log.Printf("Function Log:\n %s", string(funcLog))

	// Delete the job
	if err := a.k.DeleteFunctionJob(jobName, nsName); err != nil {
		return "", "", err
	}

	return status, string(funcLog), nil
}

func setSession(a *appContext, userName string, response http.ResponseWriter) {
	value := map[string]string{
		"name": userName,
	}
	if encoded, err := a.cookieHandler.Encode("session", value); err == nil {
		cookie := &http.Cookie{
			Name:  "session",
			Value: encoded,
			Path:  "/",
		}
		http.SetCookie(response, cookie)
	}
}

func clearSession(response http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:   "session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	}
	http.SetCookie(response, cookie)
}

func getUserName(a *appContext, request *http.Request) (userName string) {
	if cookie, err := request.Cookie("session"); err == nil {
		cookieValue := make(map[string]string)
		if err = a.cookieHandler.Decode("session", cookie.Value, &cookieValue); err == nil {
			userName = cookieValue["name"]
		}
	}
	return userName
}

func putUserIfNotExistedInDB(a *appContext, groupName, userName string) (int64, int64, error) {
	return a.dal.PutUserIfNotExisted(groupName, userName)
}

func getUserFunctions(a *appContext, username string, userId int64) ([]*FunctionRow, error) {
	functions, err := a.dal.ListFunctionsOfUser(username, userId)
	if err != nil {
		return nil, err
	}
	l := len(functions)
	funcToBeListed := make([]*FunctionRow, 0, l)
	for i := 0; i < l; i++ {
		f := &FunctionRow{
			FuncName:    functions[i].Name,
			Owner:       username,
			UpdatedTime: functions[i].Updated,
		}
		funcToBeListed = append(funcToBeListed, f)
	}
	return funcToBeListed, nil
}

func putUserFunction(a *appContext, username, funcName, funcContent string, userId int64) error {
	_, _, err := a.dal.PutFunction(username, funcName, funcContent, -1)
	return err
}

func checkCredentials(a *appContext, name string, pass string) (bool, error) {
	var l *ldap.Conn
	var err error

	servers := a.conf.LDAPcfg.LDAPServer
	port := a.conf.LDAPcfg.LDAPPort
	retries := a.conf.LDAPcfg.LDAPRetries
	username := fmt.Sprintf(a.conf.LDAPcfg.LDAPBaseDn, name)

	log.Println("Authenticating user", name)

	//Connect to LDAP servers with retries
	for i := 0; i < retries; i++ {
		for _, s := range servers {
			log.Println("Connecting to LDAP server", s, "......")
			l, err = ldap.DialTLS("tcp", fmt.Sprintf("%s:%d", s, port),
				&tls.Config{ServerName: s})
			if err == nil {
				break
			}
		}
		if err == nil {
			log.Println("Connected")
			break
		}
	}

	if err != nil {
		log.Println(err)
		return false, err
	}
	defer l.Close()

	//Bind
	err = l.Bind(username, pass)
	if err != nil {
		log.Println(err)
		return false, err
	}
	log.Printf("Bound user %s\n", name)
	return true, nil
}

// Add imports and the remaining code
func formatCode(code, functionName string) string {
	return fmt.Sprintf("import json\nimport os\n\n"+
		"%s\n\n"+
		"params = os.environ[\"SERVERLESS_PARAMS\"]\n"+
		"%s(json.loads(params))\n", code, functionName)
}
