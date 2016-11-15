package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/Symantec/Go-kexec/dal"
	"github.com/Symantec/Go-kexec/docker"
	"github.com/Symantec/Go-kexec/kexec"
	"github.com/gorilla/securecookie"
)

var (
	argConfigFile = flag.String("config", "", "Config file")
)

const (
	SERVERLESS_NAMESPACE string = "serverless"
	DAL_USERS_TABLE      string = "users"
	DAL_FUNCTIONS_TABLE  string = "functions"
	DAL_EXECUTIONS_TABLE string = "executions"
)

func main() {
	flag.Parse()
	configFile, err := ioutil.ReadFile(*argConfigFile)
	if err != nil {
		log.Fatalf("Cannot read config file %s: %v\n", *argConfigFile, err)
	}
	var conf appConfig
	err = json.Unmarshal(configFile, &conf)
	if err != nil {
		log.Fatalf("Cannot load config file %s: %v\n", *argConfigFile, err)
	}

	logfile, err := openLogFile(conf.LogFileDir)
	if err != nil {
		log.Fatalf("Cannot open log file: %v\n", err)
	}

	log.SetOutput(logfile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// cookie handling
	cookieHandler := securecookie.New(
		securecookie.GenerateRandomKey(64),
		securecookie.GenerateRandomKey(32),
	)

	// docker handler for creating function and pushing function image
	// to docker registry
	d := docker.NewDocker(
		// http headers
		conf.DockerCfg.HttpHeader,
		// docker host
		conf.DockerCfg.DockerHost,
		// docker api version
		conf.DockerCfg.ApiVersion,
		// http client
		nil,
	)

	// kubernetes handler for calling function and pulling function
	// execution logs
	k, err := kexec.NewKexec(&kexec.KexecConfig{
		KubeConfig: conf.KubeConfig,
	})

	if err != nil {
		panic(err)
	}

	// data access layer. Default MySQL
	//
	// TODO: dal should be pluggable
	dal, err := dal.NewMySQL(&dal.DalConfig{
		DBHost:   conf.DalCfg.DBHost,
		Username: conf.DalCfg.Username,
		Password: conf.DalCfg.Password,

		DBName: conf.DalCfg.DBName,

		UsersTable:      DAL_USERS_TABLE,
		FunctionsTable:  DAL_FUNCTIONS_TABLE,
		ExecutionsTable: DAL_EXECUTIONS_TABLE,
	})

	if err != nil {
		panic(err)
	}

	context := &appContext{d: d, k: k, dal: dal, cookieHandler: cookieHandler, conf: &conf}

	router := NewRouter(context)

	http.Handle("/", router)

	panic(http.ListenAndServe(":8080", nil))
}
