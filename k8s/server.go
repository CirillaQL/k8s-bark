package k8s

import (
	"flag"
	"fmt"
	"k8s-bark/bark"
	"k8s-bark/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
	"sync"
	"time"
)

type K8sWatch struct {
	config     *rest.Config          // 配置
	clientset  *kubernetes.Clientset // kubernetes 客户端
	bark       *bark.Bark            // bark 客户端
	namespaces []string              // 待监控命名空间
	Resource   map[string]sync.Map   // 资源Map存储
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
	k8swatch.Resource = make(map[string]sync.Map)
	return k8swatch
}

func (k8swatch *K8sWatch) Watch() {
	stopper := make(chan struct{})
	//go k8swatch.bark.HealthzCheck()
	go k8swatch.watchPodsStatus(stopper)
	//go k8swatch.bark.Send()
	select {}
}

func (k8swatch *K8sWatch) Push(message bark.Message) {
	k8swatch.bark.Push(message)
}

// watchPodsStatus 监控Pods的创建与删除
func (k8swatch *K8sWatch) watchPodsStatus(stopper chan struct{}) {
	defer close(stopper)
	// 初始化 informer
	factory := informers.NewSharedInformerFactory(k8swatch.clientset, 6*time.Hour)
	podInformer := factory.Core().V1().Pods()
	informer := podInformer.Informer()
	defer runtime.HandleCrash()

	// 启动 informer，list & watch
	go factory.Start(stopper)

	// 从 apiserver 同步资源，即 list
	if !cache.WaitForCacheSync(stopper, informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}

	// 创建 lister
	podLister := podInformer.Lister()
	// 从 lister 中获取所有 items
	podList, err := podLister.List(labels.Everything())
	if err != nil {
		fmt.Println(err)
	}

	for _, pod := range podList {
		resource := Resource{
			ResourceType:    "Pod",
			ResourceVersion: pod.ResourceVersion,
			Value:           pod,
		}
		s := k8swatch.Resource["Pod"]
		s.Store(pod.Name, resource)
	}

	// 使用自定义 handler
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			for _, pods := range podList {
				if pods.Name == pod.Name && pods.ResourceVersion != pod.ResourceVersion {
					fmt.Println(pod)
				} else {
					m := bark.Message{
						Type:        "Pod",
						Status:      "Adding",
						Information: fmt.Sprintf("Pod"),
					}
					k8swatch.Push(m)
					continue
				}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			fmt.Println(oldObj.(*corev1.Pod).Name)
		}, // 此处省略 workqueue 的使用
		DeleteFunc: func(interface{}) {
			fmt.Println("delete not implemented")
		},
	})
	<-stopper
}
