package kexec

import (
	"log"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	c := &KexecConfig{
		KubeConfig: "./fakekubeconfig",
	}

	k, err := NewKexec(c)
	if err != nil {
		panic(err)
	}

	funcName := "gorilla"
	uuid := "xxxxxxx-xxxxxxxx-xxxxxxxx"
	jobName := funcName + "-" + uuid
	image := "registry.paas.symcpe.com:443/xuant/gorilla"

	labels := make(map[string]string)

	k.CallFunction(jobName, image, "default", labels)

	time.Sleep(30 * time.Second)

	funcLog, err := k.GetFunctionLog(funcName, uuid, "default")
	if err != nil {
		panic(err)
	}
	log.Printf("Function Log:\n %s", string(funcLog))

}
