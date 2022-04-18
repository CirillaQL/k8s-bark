package k8s

import (
	"context"
	"flag"
	"fmt"
	"k8s-bark/bark"
	"k8s-bark/pkg/log"
	"path/filepath"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var LOG = log.LOG

type K8sWatch struct {
	config    *rest.Config
	clientset *kubernetes.Clientset
	bark      *bark.Bark
}

func NewK8sWatch(location, barkServer, barkToken string) (k8swatch *K8sWatch) {
	k8swatch = &K8sWatch{}
	// 初始化配置，检测k8s-bark是否在集群中运行
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
		LOG.Errorf("location: %s is not supported", location)
		panic("location must be in-cluster or out-cluster")
	}
	clientset, err := kubernetes.NewForConfig(k8swatch.config)
	if err != nil {
		panic(err.Error())
	}
	k8swatch.clientset = clientset
	bark := bark.NewBark(barkServer, barkToken)
	k8swatch.bark = bark
	return k8swatch
}

func (k8swatch *K8sWatch) Watch() {
	go k8swatch.bark.HealthzCheck()
	go k8swatch.watchPodsStatus()
	go k8swatch.watchK8sEvents()
	go k8swatch.bark.Send()
	for {
		pods, err := k8swatch.clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}
		m := bark.Message{
			Status:      "Pods",
			Information: fmt.Sprintf("There_are_%d_pods_in_the_cluster", len(pods.Items)),
		}
		// k8swatch.Push(m)
		LOG.Infof("%+v", m)
		fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))

		time.Sleep(10 * time.Second)
	}
}

func (k8swatch *K8sWatch) Push(message bark.Message) {
	k8swatch.bark.Push(message)
}

// watchPodsStatus 监控Pods Status
func (k8swatch *K8sWatch) watchPodsStatus() {
	for {
		pods, err := k8swatch.clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}
		for _, pod := range pods.Items {
			if pod.Status.Phase != "Running" {
				LOG.Infof("Pod Name: %s, Pod Status: %s", pod.Name, pod.Status.Phase)
			}
		}
		time.Sleep(5 * time.Second)
	}
}

// watchK8sEvents 监控K8s Events
func (k8swatch *K8sWatch) watchK8sEvents() {
	for {
		events, _ := k8swatch.clientset.CoreV1().Events("").List(context.TODO(), metav1.ListOptions{})
		for _, item := range events.Items {
			fmt.Println(item.Type)
		}
		time.Sleep(5 * time.Second)
	}
}
