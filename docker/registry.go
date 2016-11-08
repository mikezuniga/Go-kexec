package docker

// At the time of developing docker registry handling, ie, pushing
// created function to the registry, there is a bug that is not
// resolved in the master branch of "github.com/docker/docker".
// The bug can be found here:
//
//    https://github.com/docker/docker/issues/26781
//
//
// The following is just a hack, simply calling command line from
// golang code.
//
// After the community resolve the mentioned bug, the implementation
// should be replaced with real go code. At the meantime, all the
// dev regarding this issue can be found in branch "registry-test".
// Do `git chechout registry-test` to check it out.

import (
	"errors"
	"log"
	"os/exec"
)

func RegisterFunction(registry, namespace, funcName string) error {
	// cmd
	app := "docker"

	// args
	arg0 := "push"
	arg1 := registry + "/" + namespace + "/" + funcName
	cmd := exec.Command(app, arg0, arg1)
	stdout, err := cmd.Output()

	if err != nil {
		log.Printf("Failed to execute exec command `docker push` %s\n.", err)
		return errors.New("Failed to execuate exec command `docker push`")
	}

	log.Printf(string(stdout))
	return nil
}
