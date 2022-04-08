package k8s

import (
	"context"
	"flag"
	"fmt"
	"k8s-bark/bark"
	"path/filepath"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type K8sWatch struct {
	config    *rest.Config
	clientset *kubernetes.Clientset
	bark      *bark.Bark
}

func NewK8sWatch(location, barkServer string) (k8swatch *K8sWatch) {
	k8swatch = &K8sWatch{}
	if location == "in-cluster" {
		config, err := rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
		k8swatch.config = config
	} else if location == "out-cluster" {
		var kubeconfig *string
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
		}
		flag.Parse()

		// use the current context in kubeconfig
		config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			panic(err.Error())
		}
		k8swatch.config = config
	} else {
		panic("location must be in-cluster or out-cluster")
	}
	clientset, err := kubernetes.NewForConfig(k8swatch.config)
	if err != nil {
		panic(err.Error())
	}
	k8swatch.clientset = clientset
	bark := bark.NewBark(barkServer)
	k8swatch.bark = bark
	return k8swatch
}

func (k8swatch *K8sWatch) Watch() {
	go k8swatch.bark.HealthzCheck()
	for {
		pods, err := k8swatch.clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}
		fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))

		time.Sleep(10 * time.Second)
	}
}
