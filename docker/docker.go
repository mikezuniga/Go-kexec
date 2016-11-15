package docker

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	dc "github.com/fsouza/go-dockerclient"
)

var (
	IBContext     = "/tmp/faas-imagebuild-context/"
	RelDockerfile = "Dockerfile"
	ExecutionFile = "exec"
)

type Docker struct {
	client *dc.Client
}

func NewClient(endpoint string) (*Docker, error) {
	client, err := dc.NewClient(endpoint)
	return &Docker{client}, err
}

func (d *Docker) BuildFunction(registry, namespace, funcName, templateName, ctxDir string) error {
	if _, err := os.Stat(filepath.Join(ctxDir, ExecutionFile)); err != nil {
		log.Printf("Failed build function. Error: Execution file not found.")
		return errors.New("Execution file not found.")
	}

	if err := setRuntimeTemplate(templateName, ctxDir); err != nil {
		log.Printf("Failed to set up runtime template. Error:%s", err)
		return err
	}

	// Create a tar ball
	t := time.Now()
	inputbuf, outputbuf := bytes.NewBuffer(nil), bytes.NewBuffer(nil)
	tr := tar.NewWriter(inputbuf)
	defer tr.Close()

	log.Println("Building context", ctxDir)
	if err := filepath.Walk(ctxDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			log.Println("Adding file", info.Name())
			tr.WriteHeader(&tar.Header{Name: info.Name(), Size: info.Size(), ModTime: t, AccessTime: t, ChangeTime: t})
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(tr, file)
			return err
		}); err != nil {
		return err
	}

	// Build image
	opts := dc.BuildImageOptions{
		Name:         registry + "/" + namespace + "/" + funcName,
		InputStream:  inputbuf,
		OutputStream: outputbuf,
	}
	if err := d.client.BuildImage(opts); err != nil {
		return err
	}
	log.Println(string(outputbuf.Bytes()))

	return nil
}

func (d *Docker) RegisterFunction(registry, namespace, funcName string) error {
	outputbuf := bytes.NewBuffer(nil)
	opts := dc.PushImageOptions{
		Name:         registry + "/" + namespace + "/" + funcName,
		Tag:          "latest",
		Registry:     registry,
		OutputStream: outputbuf,
	}
	if err := d.client.PushImage(opts, dc.AuthConfiguration{}); err != nil {
		return err
	}
	log.Println(string(outputbuf.Bytes()))
	return nil
}

func (d *Docker) DeleteFunctionImage(registry, namespace, funcName string) error {
	opts := dc.RemoveImageOptions{
		Force: true,
	}
	if err := d.client.RemoveImageExtended(registry+"/"+namespace+"/"+funcName, opts); err != nil {
		return err
	}
	return nil
}

var python27Template = `FROM python:2.7
ADD . ./
ENTRYPOINT [ "python", "exec" ]
`

// setRuntimeEnv creates the runtime environment for building a docker image.
//
// Based on the templateName, this method will create a corresponding Dockerfile
// in the context directory (i.e. /tmp/faas-imagebuild-context/xxxx). To make the build process fast,
// runtime template should be proloaded onto the system.
//
// Now supporting Python27 only. Other template can be added easi
func setRuntimeTemplate(templateName, ctxDir string) error {
	switch templateName {
	case "python27":
		ioutil.WriteFile(filepath.Join(ctxDir, RelDockerfile), []byte(python27Template), 0644)
		return nil
	default:
		return errors.New("Runtime template " + templateName + " invalid or not supported yet.")

	}
}
