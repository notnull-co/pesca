package kubernetes

import (
	"context"
	"fmt"
	"sync"

	"github.com/notnull-co/pesca/internal/config"
	"github.com/rs/zerolog/log"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	instance k8s
	once     sync.Once
)

type k8s struct {
	k8s *kubernetes.Clientset
}

type Kubernetes interface {
	UpdateImage(namespace, deploymentName, containerName, image string) error
	IsContainerHealthy(namespace, deploymentName, containerName string) (bool, error)
}

func New() Kubernetes {
	once.Do(func() {
		conf := config.Get()
		var k8sConfig *rest.Config
		var err error
		if conf.Kubernetes.Config != "" {
			k8sConfig, err = clientcmd.BuildConfigFromFlags("", conf.Kubernetes.Config)
		} else {
			k8sConfig, err = rest.InClusterConfig()
		}

		if err != nil {
			log.Fatal().Err(err).Msg("rest client creation for kubernetes failed")
		}

		clientset, err := kubernetes.NewForConfig(k8sConfig)
		if err != nil {
			log.Fatal().Err(err).Msg("invalid configuration for kubernetes client")
		}

		instance = k8s{
			k8s: clientset,
		}
	})

	return &instance
}

func (k *k8s) UpdateImage(namespace, deploymentName, containerName, image string) error {
	deployment, err := k.k8s.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	var found bool
	for _, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name == containerName {
			found = false
			c.Image = containerName
		}
	}

	if !found {
		return fmt.Errorf("container %s/%s/%s could not be found", namespace, deploymentName, containerName)
	}

	_, err = k.k8s.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (k *k8s) IsContainerHealthy(namespace, deploymentName, containerName string) (bool, error) {
	deployment, err := k.k8s.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentAvailable && condition.Status != corev1.ConditionTrue {
			return false, nil
		}
	}

	pods, err := k.getPods(namespace, deploymentName)

	if err != nil {
		return false, err
	}

	for _, pod := range pods.Items {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			ready := containerStatus.Ready
			if !ready {
				return false, nil
			}
		}
	}

	return true, nil
}

func (k *k8s) getPods(namespace string, deploymentName string) (*corev1.PodList, error) {
	podList, err := k.k8s.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "app=" + deploymentName})
	if err != nil {
		return nil, err
	}

	return podList, nil
}
