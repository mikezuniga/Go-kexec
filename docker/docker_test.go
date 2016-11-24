package docker

import (
	"testing"
)

func TestBuildFunction(t *testing.T) {
	d, _ := NewClient("unix:///var/run/docker.sock")
	if err := d.BuildFunction("registry.paas.symcpe.com:443", "jingjing_ren", "faas:v1", "python27", "example/"); err != nil {
		t.Error(err)
	}
}

func TestRegisterFunction(t *testing.T) {
	d, _ := NewClient("unix:///var/run/docker.sock")
	if err := d.RegisterFunction("registry.paas.symcpe.com:443", "jingjing_ren", "faas:v1"); err != nil {
		t.Error(err)
	}
}
