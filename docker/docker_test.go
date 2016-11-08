package docker

import (
	"testing"
)

func TestBuildFunction(t *testing.T) {
	config := &DockerConfig{
		HttpHeaders: map[string]string{"User-Agent": "engine-api-cli-1.0"},
		Host:        "unix:///var/run/docker.sock",
		Version:     "v1.22",
		HttpClient:  nil,
	}

	d := NewDocker(config)

	d.BuildFunction("xuant", "faas:v1", "python27")
}

func TestRegisterFunction(t *testing.T) {
	RegisterFunction("xuant", "faas:v1")
}
