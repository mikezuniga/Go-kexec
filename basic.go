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

	newCode := formatCode(runtime, code, functionName)
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

	functionNameLower := strings.ToLower(functionName)
	// Build funtion
	if err = a.d.BuildFunction(a.conf.DockerCfg.DockerRegistry, userName, functionNameLower, runtime, ctxDir); err != nil {
		log.Println("Build function failed")
		return err
	}

	// Register function to configured docker registry
	if err = a.d.RegisterFunction(a.conf.DockerCfg.DockerRegistry, userName, functionNameLower); err != nil {
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
	var status, funcLog string

	// create a uuid for each function call. This uuid can be
	// seen as the execution id for the function (notice there
	// are multiple executions for a single function)
	uuid, err := uuid.NewTimeBased()

	if err != nil {
		log.Println("Failed to create uuid for function call.")
		return "", "", err
	}

	uuidStr := uuid.String()

	nsName := SERVERLESS_NAMESPACE
	functionNameLower := strings.ToLower(functionName)
	jobName := functionNameLower + "-" + strings.Replace(userName, "_", "-", -1) + "-" + uuidStr
	image := a.conf.DockerCfg.DockerRegistry + "/" + userName + "/" + functionNameLower
	labels := make(map[string]string)

	if err := a.k.CreateFunctionJob(jobName, image, params, nsName, labels); err != nil {
		log.Println("Failed to call function", functionName)
		return "", "", err
	}
	// Run the job
	err = a.k.RunJob(jobName, nsName)
	if err != nil {
		goto delete
	}

	// Get the log
	status, funcLog, err = a.k.GetFunctionLog(jobName, nsName)
	if err != nil {
		goto delete
	}
	log.Printf("Function Log:\n %s", funcLog)

	// Delete the job
delete:
	err2 := a.k.DeleteFunctionJob(jobName, nsName)
	if err2 != nil && err == nil {
		return "", "", err2
	} else if err2 != nil && err != nil {
		return "", "", errors.New(err.Error() + "\n" + err2.Error())
	} else if err2 == nil && err != nil {
		return "", "", err
	}

	return status, funcLog, nil
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

	servers := a.conf.LDAPCfg.LDAPServer
	port := a.conf.LDAPCfg.LDAPPort
	retries := a.conf.LDAPCfg.LDAPRetries
	username := fmt.Sprintf(a.conf.LDAPCfg.LDAPBaseDn, name)

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

const python27Tmpl = `%s

import json
import os
import sys 
import traceback

params = os.environ["SERVERLESS_PARAMS"]

try:
    p = json.loads(params)
except ValueError as e:
    print 'Parameters are not in valid json format:', e
    sys.exit(1)
except:
    print e
    sys.exit(1)

try:
    %s(p)
except NameError as e:
    print e
    sys.exit(1)
except:
    exc_type, exc_value, exc_traceback = sys.exc_info()
    tr = traceback.extract_tb(exc_traceback)
    for item in tr[1:]:
        print "line", str(item[1]), "in", item[2], "\n\t", item[3]
    print traceback.format_exc().splitlines()[-1]
    sys.exit(1)
`

// Add imports and the remaining code
func formatCode(runtime, code, functionName string) string {
	switch runtime {
	case "python27":
		return fmt.Sprintf(python27Tmpl, code, functionName)
	default:
		return fmt.Sprintf(python27Tmpl, code, functionName)
	}
}

func openLogFile(dir string) (*os.File, error) {
	_, err := os.Stat(dir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if os.IsNotExist(err) {
		if err := os.Mkdir(dir, os.ModePerm); err != nil {
			return nil, err
		}
	}
	logfile, err := os.OpenFile(filepath.Join(dir, "serverless.log"), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}
	return logfile, nil
}
