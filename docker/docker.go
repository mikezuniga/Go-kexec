package docker

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/dockerignore"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/fileutils"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
)

var (
	IBContext     = "/tmp/faas-imagebuild-context/"
	RelDockerfile = "Dockerfile"
	ExecutionFile = "exec"
)

type Docker struct {
	HttpHeaders map[string]string
	Host        string
	Version     string
	HttpClient  *http.Client
}

func NewDocker(httpHeader map[string]string, host, version string, httpClient *http.Client) *Docker {
	return &Docker{
		HttpHeaders: httpHeader,
		Host:        host,
		Version:     version,
		HttpClient:  httpClient,
	}
}

func (d *Docker) initCli() (*client.Client, error) {
	return client.NewClient(d.Host, d.Version, d.HttpClient, d.HttpHeaders)
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

	f, err := os.Open(filepath.Join(ctxDir, ".dockerignore"))

	if err != nil && !os.IsNotExist(err) {
		return err
	}
	defer f.Close()

	var excludes []string
	if err == nil {
		excludes, err = dockerignore.ReadAll(f)
		if err != nil {
			return err
		}
	}

	if err := builder.ValidateContextDirectory(ctxDir, excludes); err != nil {
		return fmt.Errorf("Error checking context: '%s'.", err)
	}

	var includes = []string{"."}
	keepThem1, _ := fileutils.Matches(".dockerignore", excludes)
	keepThem2, _ := fileutils.Matches(RelDockerfile, excludes)
	if keepThem1 || keepThem2 {
		includes = append(includes, ".dockerignore", RelDockerfile)
	}

	buildCtx, err := archive.TarWithOptions(ctxDir, &archive.TarOptions{
		Compression:     archive.Uncompressed,
		ExcludePatterns: excludes,
		IncludeFiles:    includes,
	})

	if err != nil {
		return err
	}

	progressOutput := streamformatter.NewStreamFormatter().NewProgressOutput(bytes.NewBuffer(nil), true)
	var body io.Reader = progress.NewProgressReader(buildCtx, progressOutput, 0, "", "Sending build context to Docker daemon")

	opts := types.ImageBuildOptions{
		Tags:       []string{registry + "/" + namespace + "/" + funcName},
		Dockerfile: RelDockerfile,
		Squash:     true,
	}

	cli, err := d.initCli()

	if err != nil {
		log.Printf("Failed to init cli. Error: %s", err)
		return err
	}

	resp, err := cli.ImageBuild(context.Background(), body, opts)

	if err != nil {
		log.Printf("Failed to build image. Error: %s", err)
		return err
	}
	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Printf("Failed to read response body. Error: %s", err)
		return err
	}

	log.Printf("Image build response Body: %s", string(buf))
	log.Printf("Image build response OSType: %s", resp.OSType)

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
