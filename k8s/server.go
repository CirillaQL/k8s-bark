package k8s

import (
	"context"
	"flag"
	"fmt"
	"k8s-bark/bark"
	"k8s-bark/pkg/log"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
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
	stopper := make(chan struct{})
	go k8swatch.bark.HealthzCheck()
	go k8swatch.watchPodsStatus(stopper)
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

		time.Sleep(10 * time.Second)
	}
}

func (k8swatch *K8sWatch) Push(message bark.Message) {
	k8swatch.bark.Push(message)
}

// watchPodsStatus 监控Pods的创建与删除
func (k8swatch *K8sWatch) watchPodsStatus(stopper chan struct{}) {
	// 初始化informer
	podFactory := informers.NewSharedInformerFactory(k8swatch.clientset, 3*time.Hour)
	podInformer := podFactory.Core().V1().Pods().Informer()
	go podFactory.Start(stopper)

	// 从apiserver 同步资源，即List
	if !cache.WaitForCacheSync(stopper, podInformer.HasSynced) {
		LOG.Error("Timed out waiting for caches to sync")
		return
	}

	// 使用自定义Handler
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			m := bark.Message{
				Status:      "Pods",
				Information: fmt.Sprintf("Pod_%s_is_created", pod.Name),
			}
			LOG.Infof("%+v", m)
		},
		UpdateFunc: func(old, new interface{}) {
			old_pod := old.(*v1.Pod)
			new_pod := new.(*v1.Pod)
			if old_pod.ResourceVersion == new_pod.ResourceVersion {
				LOG.Info("Pod is not changed")
			} else {
				m := bark.Message{
					Status:      "Pods",
					Information: fmt.Sprintf("Pod_%s_is_updated", new_pod.Name),
				}
				LOG.Infof("%+v", m)
			}
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			m := bark.Message{
				Status:      "Pods",
				Information: fmt.Sprintf("Pod_%s_is_deleted", pod.Name),
			}
			LOG.Infof("%+v", m)
		},
	})
}
