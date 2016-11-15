package docker

import (
	"testing"
)

func TestBuildFunction(t *testing.T) {
	d, _ := docker.NewClient("unix:///var/run/docker.sock")
	d.BuildFunction("xuant", "faas:v1", "python27")
}

func TestRegisterFunction(t *testing.T) {
	d, _ := docker.NewClient("unix:///var/run/docker.sock")
	d.RegisterFunction("xuant", "faas:v1")
}
