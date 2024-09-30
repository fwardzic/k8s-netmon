package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-ping/ping"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	pingStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "network_test_ping_status",
			Help: "Ping test status (1 = success, 0 = failure)",
		},
		[]string{"nodeName"},
	)
	kubeApiStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "network_test_k8s_api_status",
			Help: "Kubernetes API test status (1 = success, 0 = failure)",
		},
		[]string{"nodeName"},
	)
	externalUrlStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "network_test_external_http_status",
			Help: "External HTTP test status (1 = success, 0 = failure)",
		},
		[]string{"nodeName"},
	)
)

func init() {
	// Register metrics with Prometheus
	prometheus.MustRegister(pingStatus)
	prometheus.MustRegister(kubeApiStatus)
	prometheus.MustRegister(externalUrlStatus)
}

func getPodIP(namespace string) []string {
	var podIpList []string
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("Could not get cluster config %v\n", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("Could not create new clientset %v\n", err)
	}
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Could not get list of pods %v\n", err)
	}
	for _, pod := range pods.Items {
		if pod.Status.PodIP == "" {
			log.Printf("Missing IP address for Pod: %s", pod.Name)
		} else if pod.Status.PodIP != "" {
			podIpList = append(podIpList, pod.Status.PodIP)
		}
	}
	log.Printf("list of IPs: %s", podIpList)
	return podIpList
}

func pingIp(ip string, nodeName string, wg *sync.WaitGroup) {
	defer wg.Done()
	pinger, err := ping.NewPinger(ip)
	if err != nil {
		log.Printf("Could not create new ping instance for %s: ERROR: %v\n", ip, err)
	}
	pinger.SetPrivileged(true)
	pinger.Count = 5
	pinger.Timeout = time.Second * 1
	err = pinger.Run()
	if err != nil {
		log.Printf("Failed to ping IP %s: %v", ip, err)
		pingStatus.WithLabelValues(nodeName).Set(0)
	} else {
		log.Printf("Successfully pinged IP %s: %v", ip, err)
		pingStatus.WithLabelValues(nodeName).Set(1)
	}
}

func kubeApiTest(nodeName string, wg *sync.WaitGroup) {
	defer wg.Done()
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	url := "https://kubernetes.default.svc"
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Could not connect to Kubernetes service. %s\n", err)
		kubeApiStatus.WithLabelValues(nodeName).Set(0)
	} else {
		log.Printf("Successfully connected to Kubernetes service %d\n", resp.StatusCode)
		defer resp.Body.Close()
		kubeApiStatus.WithLabelValues(nodeName).Set(1)
	}
}

func externalUrlTest(url string, nodeName string, wg *sync.WaitGroup) {
	defer wg.Done()
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Could not connect to External service. %s\n", err)
		externalUrlStatus.WithLabelValues(nodeName).Set(0)
	} else {
		log.Printf("Successfully connected to External service %d\n", resp.StatusCode)
		defer resp.Body.Close()
		externalUrlStatus.WithLabelValues(nodeName).Set(1)
	}
}

func main() {

	ns := "netmon"

	// Serve Prometheus metrics
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Println("Serving metrics at /metrics")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		log.Fatal("NODE_NAME environment variable not set")
	}

	for {
		PodIpList := getPodIP(ns)

		var wg sync.WaitGroup

		for _, ip := range PodIpList {
			wg.Add(1)
			go pingIp(ip, nodeName, &wg)
		}

		wg.Add(1)
		go kubeApiTest(nodeName, &wg)

		externalUrl := os.Getenv("EXTERNAL_URL")
		if externalUrl == "" {
			log.Fatal("External URL is not set")
		}
		wg.Add(1)
		go externalUrlTest(externalUrl, nodeName, &wg)

		wg.Wait()
		time.Sleep(60 * time.Second)
	}
}
