package k8s

import (
	"flag"
	"fmt"
	"k8s-bark/bark"
	"k8s-bark/pkg/logger"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type K8sWatch struct {
	config     *rest.Config          // 配置
	clientset  *kubernetes.Clientset // kubernetes 客户端
	bark       *bark.Bark            // bark 客户端
	namespaces []string              // 待监控命名空间
}

func NewK8sWatch(location, barkServer, barkToken string, namespaces []string) (k8swatch *K8sWatch) {
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
		logger.Log().Errorf("location: %s is not supported", location)
		panic("location must be in-cluster or out-cluster")
	}
	clientset, err := kubernetes.NewForConfig(k8swatch.config)
	if err != nil {
		panic(err.Error())
	}
	k8swatch.clientset = clientset
	bark := bark.NewBark(barkServer, barkToken)
	k8swatch.bark = bark
	k8swatch.namespaces = namespaces
	return k8swatch
}

func (k8swatch *K8sWatch) Watch() {
	stopper := make(chan struct{})
	go k8swatch.bark.HealthzCheck()
	go k8swatch.watchPodsStatus(stopper)
	go k8swatch.bark.Send()
	select {}
}

func (k8swatch *K8sWatch) Push(message bark.Message) {
	k8swatch.bark.Push(message)
}

// watchPodsStatus 监控Pods的创建与删除
func (k8swatch *K8sWatch) watchPodsStatus(stopper chan struct{}) {
	// 初始化informer
	podFactory := informers.NewSharedInformerFactory(k8swatch.clientset, 3*time.Hour)
	podInformer := podFactory.Core().V1().Pods()
	informer := podInformer.Informer()
	informer.Run(stopper)

	// 从apiserver 同步资源，即List
	if !cache.WaitForCacheSync(stopper, informer.HasSynced) {
		logger.Log().Error("Timed out waiting for caches to sync")
		return
	}

	podLister := podInformer.Lister()
	podInitList, err := podLister.List(labels.Everything())
	if err != nil {
		logger.Log().Errorf("List pods failed: %s", err.Error())
	}

	// 使用自定义Handler
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			new_pod := obj.(*v1.Pod)
			for _, podInList := range podInitList {
				if podInList.Name == new_pod.Name {
					if podInList.ResourceVersion == new_pod.ResourceVersion {
						return
					} else {
						m := bark.Message{
							Type:        "Pod",
							Status:      "Add",
							Information: fmt.Sprintf("Pod_%s_is_created", new_pod.Name),
						}
						logger.Log().Infof("%+v", m)
					}
				}
			}
		},
		UpdateFunc: func(old, new interface{}) {
			old_pod := old.(*v1.Pod)
			new_pod := new.(*v1.Pod)
			if old_pod.ResourceVersion != new_pod.ResourceVersion {
				m := bark.Message{
					Type:        "Pod",
					Status:      "Update",
					Information: fmt.Sprintf("Pod_%s_is_updated", new_pod.Name),
				}
				logger.Log().Infof("%+v", m)
			}
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			m := bark.Message{
				Type:        "Pod",
				Status:      "Delete",
				Information: fmt.Sprintf("Pod_%s_is_deleted", pod.Name),
			}
			logger.Log().Infof("%+v", m)
		},
	})

}
