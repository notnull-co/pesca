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

// func updateDeploymentImage(clientset *kubernetes.Clientset, namespace, deploymentName, newImage string) error {
// 	deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
// 	if err != nil {
// 		return err
// 	}

// 	// Update the container image in the deployment
// 	deployment.Spec.Template.Spec.Containers[0].Image = newImage

// 	_, err = clientset.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }
