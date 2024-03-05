package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/notnull-co/pesca/internal/config"
	"github.com/notnull-co/pesca/internal/domain"
	"github.com/rs/zerolog/log"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
)

var (
	instance k8s
	once     sync.Once
)

type k8s struct {
	k8s *kubernetes.Clientset
}

type DeploymentUpdate struct {
	Old map[domain.Deployment]string
	New map[domain.Deployment]string
}

type Kubernetes interface {
	UpdateImage(isca domain.Isca, revision domain.ImageRevision) error
	IsContainerHealthy(namespace, deploymentName, containerName, image string) (bool, error)
	WatchDeployments(annotationFilter map[string]string, updated chan *DeploymentUpdate, created chan map[domain.Deployment]string, deleted chan map[domain.Deployment]string) (chan struct{}, error)
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

func (k *k8s) UpdateImage(isca domain.Isca, revision domain.ImageRevision) error {
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		deployment, getErr := k.k8s.AppsV1().Deployments(isca.Deployment.Namespace).Get(context.TODO(), isca.Deployment.Name, metav1.GetOptions{})
		if getErr != nil {
			return getErr
		}
		var found bool
		for _, c := range deployment.Spec.Template.Spec.Containers {
			if c.Name == isca.Deployment.ContainerName {
				found = false
				c.Image = isca.Registry.RegistryURL + isca.Registry.Repository + ":" + revision.Version
			}
		}
		if !found {
			return fmt.Errorf("container %s/%s/%s could not be found", isca.Deployment.Namespace, isca.Deployment.Name, isca.Deployment.ContainerName)
		}
		_, updateErr := k.k8s.AppsV1().Deployments(isca.Deployment.Namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
		return updateErr
	})
	if retryErr != nil {
		return retryErr
	}

	return nil
}

func (k *k8s) IsContainerHealthy(namespace, deploymentName, containerName, image string) (bool, error) {
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
			if containerStatus.Name == containerName && containerStatus.Image == image && !containerStatus.Ready {
				return false, nil
			}
		}
	}

	return true, nil
}

func (k *k8s) WatchDeployments(annotationFilter map[string]string, updated chan *DeploymentUpdate, created chan map[domain.Deployment]string, deleted chan map[domain.Deployment]string) (chan struct{}, error) {
	informer := cache.NewSharedInformer(
		cache.NewListWatchFromClient(
			k.k8s.AppsV1().RESTClient(),
			"deployments",
			metav1.NamespaceAll,
			fields.Everything(),
		),
		&appsv1.Deployment{},
		time.Minute,
	)

	_, err := informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				deployment, ok := obj.(*appsv1.Deployment)

				if !isValid(deployment, annotationFilter) {
					return
				}

				if ok {
					images := mapDeploymentToDomain(deployment)
					log.Debug().Any("deployment", logFormat(images)).Msg("deployment sent to the created channel")
					created <- images
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				newDeployment, okNew := newObj.(*appsv1.Deployment)
				oldDeployment, okOld := oldObj.(*appsv1.Deployment)

				if !isValid(oldDeployment, annotationFilter) && !isValid(newDeployment, annotationFilter) {
					return
				}

				if okNew && okOld {
					imagesNew := mapDeploymentToDomain(newDeployment)
					imagesOld := mapDeploymentToDomain(oldDeployment)

					log.Debug().Any("deployment_new", logFormat(imagesNew)).Any("deployment_old", logFormat(imagesOld)).Msg("deployment sent to the updated channel")
					updated <- &DeploymentUpdate{
						Old: imagesOld,
						New: imagesNew,
					}
				}
			},
			DeleteFunc: func(obj interface{}) {
				deployment, ok := obj.(*appsv1.Deployment)

				if !isValid(deployment, annotationFilter) {
					return
				}

				if ok {
					images := mapDeploymentToDomain(deployment)
					log.Debug().Any("deployment", logFormat(images)).Msg("deployment sent to the deleted channel")
					deleted <- images
				}
			},
		},
	)

	if err != nil {
		return nil, err
	}

	stop := make(chan struct{}, 1)
	go informer.Run(stop)
	return stop, nil
}

func logFormat(deployment map[domain.Deployment]string) []struct {
	domain.Deployment
	Image string
} {
	var formated []struct {
		domain.Deployment
		Image string
	}

	for deployment, image := range deployment {
		formated = append(formated, struct {
			domain.Deployment
			Image string
		}{
			Deployment: deployment,
			Image:      image,
		})
	}

	return formated
}

func mapDeploymentToDomain(deployment *appsv1.Deployment) map[domain.Deployment]string {
	deploymentImages := make(map[domain.Deployment]string, len(deployment.Spec.Template.Spec.Containers))
	for _, container := range deployment.Spec.Template.Spec.Containers {
		deploymentImages[domain.Deployment{
			Name:          deployment.ObjectMeta.Name,
			Namespace:     deployment.ObjectMeta.Namespace,
			ContainerName: container.Name,
			Active:        strings.ToLower(deployment.Annotations["pescar"]) == "true",
			Image:         container.Image,
		}] = container.Image
	}
	return deploymentImages
}

func (k *k8s) getPods(namespace string, deploymentName string) (*corev1.PodList, error) {
	podList, err := k.k8s.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "app=" + deploymentName})
	if err != nil {
		return nil, err
	}

	return podList, nil
}

func isValid(deployment *appsv1.Deployment, filters map[string]string) bool {
	for key, value := range filters {
		if strings.EqualFold(deployment.Annotations[key], value) {
			return false
		}
	}

	return true
}
