package kubernetes

import (
	"fmt"
	"testing"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func init() {
	k8sConfig, err := clientcmd.BuildConfigFromFlags("", "/home/murillovaz/.kube/config")

	if err != nil {
		panic(err)
	}

	clientset, err := kubernetes.NewForConfig(k8sConfig)

	if err != nil {
		panic(err)
	}

	instance = k8s{
		k8s: clientset,
	}
}

func TestHealthy(t *testing.T) {
	healthy, err := instance.IsContainerHealthy("kube-system", "coredns", "core")

	if err != nil {
		panic(err)
	}
	fmt.Printf("healthy: %v", healthy)
}
