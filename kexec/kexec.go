/*
Copyright 2016 Xuan Tang. All rights reserved.
Use of this source code is governed by a license
that can be found in the LICENSE file.
*/

package kexec

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"k8s.io/client-go/1.4/kubernetes"
	"k8s.io/client-go/1.4/pkg/api"
	unversioned "k8s.io/client-go/1.4/pkg/api/unversioned"
	v1 "k8s.io/client-go/1.4/pkg/api/v1"
	batchv1 "k8s.io/client-go/1.4/pkg/apis/batch/v1"
	"k8s.io/client-go/1.4/pkg/labels"
	"k8s.io/client-go/1.4/tools/clientcmd"
)

var (
	JobEnvParams                 = "SERVERLESS_PARAMS"
	MaxPodExecTime time.Duration = 120
)

type KexecConfig struct {
	KubeConfig string
}

type Kexec struct {
	Clientset *kubernetes.Clientset
}

// NewKexec creates a new Kexec instance which contains all the methods
// to communicate with the kubernetes/openshift cluster.
func NewKexec(c *KexecConfig) (*Kexec, error) {
	config, err := clientcmd.BuildConfigFromFlags("", c.KubeConfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Kexec{
		Clientset: clientset,
	}, nil
}

// Create a job template and then create the job
// instance against the specified kubernetes/openshift cluster.
//
// Returns:		(error) if there is one
func (k *Kexec) CreateFunctionJob(jobname, image, params, namespace string, labels map[string]string) error {
	template := createJobTemplate(image, jobname, params, namespace, labels)

	_, err := k.Clientset.Batch().Jobs(namespace).Create(template)
	if err != nil {
		return err
	}

	return nil
}

// Get the log for the job.
// This function loop over all pods created by the job and return
// the log of the first non-pending and non-running one.
//
// Returns: (string) pod status
//			([]byte) pod log
//			(error) if there is one
// TODO: Logs should be return in full if there are multiple pods
//       for one function execution.
func (k *Kexec) GetFunctionLog(jobName, namespace string) (string, string, error) {

	podlist, err := k.getFunctionPods(jobName, namespace)
	if err != nil {
		return "", "", err
	}

	if len(podlist.Items) < 1 {
		return "", "", errors.New(fmt.Sprintf("No pod found for job %s.", jobName))
	}

	// Get the name and status of the first non-pending and non-running pod
	var podName string
	var podPhase v1.PodPhase
	for _, pod := range podlist.Items {
		if pod.Status.Phase != v1.PodPending &&
			pod.Status.Phase != v1.PodRunning {
			podName = pod.Name
			podPhase = pod.Status.Phase
			break
		}
	}
	if podName == "" {
		return "", "", errors.New(fmt.Sprintf("No completed pod for job %s.", jobName))
	}

	opts := &v1.PodLogOptions{
		Follow:     true,
		Timestamps: false,
	}

	response, err := k.Clientset.Core().Pods(namespace).GetLogs(podName, opts).Stream()

	if err != nil {
		return "", "", err
	}

	defer response.Close()

	log.Println("Got log of pod", podName)

	status := string(podPhase)
	log.Println("Job", jobName, "status:", status)

	res, err := ioutil.ReadAll(response)
	return status, string(res), err
}

// Get pod(s) that ran a specific function execution (job).
//
// Returns:	pod list
func (k *Kexec) GetFunctionPods(jobName, namespace string) (*v1.PodList, error) {
	return k.getFunctionPods(jobName, namespace)
}

// Wait for job to complete and delete the job.
// Note in Kubernetes when a Pod fails, then the Job controller starts a new Pod.
// The current implementation waits for the first pod completes and exits.
func (k *Kexec) RunJob(jobName, namespace string) error {
	// Wait for first pod completes
	_, err := k.waitForPodComplete(jobName, namespace)
	if err != nil {
		return err
	}
	return nil
}

// Delete the entire job and its pods
func (k *Kexec) DeleteFunctionJob(jobName, namespace string) error {
	log.Println("Deleting job", jobName, "and its pods...")
	var deleteOrphanDep = true
	deleteOptions := api.DeleteOptions{
		OrphanDependents: &deleteOrphanDep,
	}
	if err := k.Clientset.Batch().Jobs(namespace).Delete(jobName, &deleteOptions); err != nil {
		return err
	}
	if err := k.DeleteFunctionPods(jobName, namespace); err != nil {
		return err
	}
	return nil
}

// Delete all pods for a specific job
func (k *Kexec) DeleteFunctionPods(jobName, namespace string) error {
	var deleteOrphanDep = true
	deleteOptions := api.DeleteOptions{
		OrphanDependents: &deleteOrphanDep,
	}
	// Create job label selector
	jobLabelSelector := labels.SelectorFromSet(labels.Set{
		"job-name": jobName,
	})

	// List pods according to `jobLabelSelector`
	listOptions := api.ListOptions{
		LabelSelector: jobLabelSelector,
	}

	return k.Clientset.Core().Pods(namespace).DeleteCollection(&deleteOptions, listOptions)
}

// Create a namespace if it does not exist
func (k *Kexec) CreateUserNamespaceIfNotExist(namespace string) (*v1.Namespace, error) {
	if ns, err := k.Clientset.Core().Namespaces().Get(namespace); err == nil {
		log.Println("Namespace", namespace, "already exists!")
		return ns, nil
	}
	return k.createNamespace(namespace)
}

func (k *Kexec) waitForPodComplete(jobName, namespace string) (v1.PodPhase, error) {
	// Create job label selector
	jobLabelSelector := labels.SelectorFromSet(labels.Set{
		"job-name": jobName,
	})

	// List pods according to `jobLabelSelector`
	listOptions := api.ListOptions{
		Watch:         true,
		LabelSelector: jobLabelSelector,
	}

	var podPhase v1.PodPhase
	w, err := k.Clientset.Core().Pods(namespace).Watch(listOptions)
	if err != nil {
		return podPhase, err
	}
	func() {
		for {
			select {
			case events, ok := <-w.ResultChan():
				if !ok {
					return
				}
				resp := events.Object.(*v1.Pod)
				podPhase = resp.Status.Phase
				log.Println("Pod status:", podPhase)
				if podPhase != v1.PodPending &&
					podPhase != v1.PodRunning {
					w.Stop()
				}
				if podPhase == v1.PodUnknown {
					log.Println("Pod status unknown. Reason:", resp.Status.Reason)
				}
			case <-time.After(MaxPodExecTime * time.Second):
				err = errors.New("Function takes too long to complete.")
				w.Stop()
			}
		}
	}()
	return podPhase, err
}

// private function to help get the exact pod(s) that ran a specific
// function execution.
func (k *Kexec) getFunctionPods(jobName, namespace string) (*v1.PodList, error) {
	// Create job label selector
	jobLabelSelector := labels.SelectorFromSet(labels.Set{
		"job-name": jobName,
	})

	// List pods according to `jobLabelSelector`
	listOptions := api.ListOptions{
		LabelSelector: jobLabelSelector,
	}

	return k.Clientset.Core().Pods(namespace).List(listOptions)
}

// private function to create a namespace
func (k *Kexec) createNamespace(namespace string) (*v1.Namespace, error) {
	labels := make(map[string]string)
	labels["name"] = namespace
	ns := &v1.Namespace{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:   namespace,
			Labels: labels,
		},
	}
	return k.Clientset.Core().Namespaces().Create(ns)
}

// createJobTemplate create a Job template, which will be used to
// create a Job instance against the specified kubernetes/openshift
// cluster.
//
// For now, user only provide image, jobname, namespace and labels.
// Other features like parallelism, etc., cannot be specified.
//
// TODO: 1. make parallelism configurable
func createJobTemplate(image, jobname, params, namespace string, labels map[string]string) *batchv1.Job {
	if params == "" {
		params = "{}"
	}

	return &batchv1.Job{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Job",
			APIVersion: "batch/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      jobname,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: v1.ObjectMeta{
					Name: jobname,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						v1.Container{
							Name:  jobname,
							Image: image,
							Env: []v1.EnvVar{
								v1.EnvVar{
									Name:  JobEnvParams,
									Value: params,
								},
							},
						},
					},
					RestartPolicy: v1.RestartPolicyNever,
				},
			},
		},
	}
}
