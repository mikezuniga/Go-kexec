package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/Symantec/Go-kexec/dal"
	"github.com/Symantec/Go-kexec/docker"
	"github.com/Symantec/Go-kexec/kexec"
	"github.com/gorilla/securecookie"
)

var (
	argConfigFile        = flag.String("config", "", "Config file")
	SERVERLESS_NAMESPACE = "serverless"
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

	// cookie handling
	cookieHandler := securecookie.New(
		securecookie.GenerateRandomKey(64),
		securecookie.GenerateRandomKey(32),
	)

	// docker handler for creating function and pushing function image
	// to docker registry
	d := docker.NewDocker(
		// http headers
		map[string]string{"User-Agent": "engin-api-cli-1.0"},
		// docker host
		"unix:///var/run/docker.sock",
		// docker api version
		"v1.22",
		// http client
		nil,
	)

	// kubernetes handler for calling function and pulling function
	// execution logs
	k, err := kexec.NewKexec(&kexec.KexecConfig{
		KubeConfig: os.Getenv("HOME") + "/.kube/config",
	})

	if err != nil {
		panic(err)
	}

	// data access layer. Default MySQL
	//
	// TODO: dal should be pluggable
	dal, err := dal.NewMySQL(&dal.DalConfig{
		DBHost:   "100.73.145.91",
		Username: "kexec",
		Password: "password",

		DBName: "kexec",

		UsersTable:      "users",
		FunctionsTable:  "functions",
		ExecutionsTable: "executions",
	})

	if err != nil {
		panic(err)
	}

	context := &appContext{d: d, k: k, dal: dal, cookieHandler: cookieHandler, conf: &conf}

	router := NewRouter(context)

	http.Handle("/", router)

	panic(http.ListenAndServe(":8080", nil))
}
