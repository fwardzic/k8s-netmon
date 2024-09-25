package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/go-ping/ping"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	avgRtt = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ping_avg_rtt_seconds",
			Help: "Average round-trip time of ICMP pings in seconds.",
		},
		[]string{"ip"},
	)
	packetLoss = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ping_packet_loss_percent",
			Help: "Percentage of packet loss during ICMP pings.",
		},
		[]string{"ip"},
	)
	pingCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ping_count_total",
			Help: "Total number of ICMP pings sent.",
		},
		[]string{"ip"},
	)
)

func init() {
	// Register metrics with Prometheus
	prometheus.MustRegister(avgRtt)
	prometheus.MustRegister(packetLoss)
	prometheus.MustRegister(pingCount)
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

func pingIp(PodIpList []string) {
	for _, ip := range PodIpList {
		pinger, err := ping.NewPinger(ip)
		if err != nil {
			log.Printf("Could not create new ping instance. ERROR: %v\n", err)
			continue
		}
		pinger.SetPrivileged(true)
		pinger.Count = 2
		pinger.Timeout = time.Second * 1
		err = pinger.Run()
		if err != nil {
			log.Printf("Failed to ping IP %s: %v", ip, err)
			continue
		}
		// Collect ping statistics
		stats := pinger.Statistics()
		log.Printf("Ping statistics for %s: %+v\n", ip, stats)

		// Update Prometheus metrics
		avgRtt.WithLabelValues(ip).Set(stats.AvgRtt.Seconds())
		packetLoss.WithLabelValues(ip).Set(stats.PacketLoss)
		pingCount.WithLabelValues(ip).Add(float64(pinger.Count))
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

	for {
		var PodIpList []string
		PodIpList = getPodIP(ns)
		pingIp(PodIpList)
		time.Sleep(60 * time.Second)
	}
}
