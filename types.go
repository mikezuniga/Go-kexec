package main

import (
	"net/http"
	"time"

	"github.com/Symantec/Go-kexec/dal"
	"github.com/Symantec/Go-kexec/docker"
	"github.com/Symantec/Go-kexec/kexec"
	"github.com/gorilla/securecookie"
)

// Error represents a handler error. It provides methods for a HTTP status
// code and embeds the built-in error interface.
type Error interface {
	error
	Status() int
	Message() string
	SendErrorResponse() bool
}

// StatusError represents an error with an associated HTTP status code.
type StatusError struct {
	Code        int
	Err         error
	UserMsg     string
	SendErrResp bool
}

// Allows StatusError to satisfy the error interface.
func (se StatusError) Error() string {
	return se.Err.Error()
}

// Returns our HTTP status code.
func (se StatusError) Status() int {
	return se.Code
}

func (se StatusError) Message() string {
	return se.UserMsg
}

func (se StatusError) SendErrorResponse() bool {
	return se.SendErrResp
}

type appConfig struct {
	FileServerDir string
	LogFileDir    string
	KubeConfig    string
	DockerCfg     dockerConfig
	DalCfg        dalConfig
	LDAPCfg       ldapConfig
}

type dockerConfig struct {
	HttpHeader     map[string]string
	DockerHost     string
	ApiVersion     string
	DockerRegistry string
}

type dalConfig struct {
	DBHost   string
	Username string
	Password string
	DBName   string
}

type ldapConfig struct {
	LDAPServer  []string
	LDAPPort    int
	LDAPRetries int
	LDAPBaseDn  string
}

type appContext struct {
	d             *docker.Docker
	k             *kexec.Kexec
	dal           dal.DAL
	cookieHandler *securecookie.SecureCookie
	conf          *appConfig
}

type appRouteHandler func(*appContext, http.ResponseWriter, *http.Request) error

type appHandler struct {
	*appContext
	H appRouteHandler
}

type LoginPage struct {
	LoginErr bool
	ErrMsg   string
}

type FunctionRow struct {
	FuncName    string
	Owner       string
	UpdatedTime time.Time
}

type DashboardPage struct {
	Username  string
	Functions []*FunctionRow
}

type CallResult struct {
	Result string
	Log    string
}

type ConfigFuncPage struct {
	EnableFuncName bool
	FuncName       string
	FuncRuntime    string
	FuncContent    string
}

type ErrorPage struct {
	Message string
}
