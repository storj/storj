// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package k8s

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"storj.io/common/time2"
)

var (
	podNamespace        = os.Getenv("MY_POD_NAMESPACE")
	podDeployment, _, _ = parsePod(os.Getenv("MY_POD_NAME"))
	podRefreshInterval  = mustParseDuration(
		os.Getenv("STORJ_POD_COUNT_REFRESH_INTERVAL"),
		5*time.Minute)

	initOnce  sync.Once
	podCount  atomic.Int32
	clientset *kubernetes.Clientset
)

func mustParseDuration(intervalStr string, defaultVal time.Duration) time.Duration {
	if intervalStr == "" {
		return defaultVal
	}
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		panic(fmt.Sprintf("failed to parse %q: %+v", intervalStr, err))
	}
	return interval
}

func clamp(x, min, max float64) float64 {
	if x < min {
		return min
	}
	if x > max {
		return max
	}
	return x
}

func jitter(t time.Duration) time.Duration {
	nanos := clamp(rand.NormFloat64()*float64(t/4), -float64(t), float64(t)) + float64(t)
	if nanos <= 0 {
		nanos = 1
	}
	return time.Duration(nanos)
}

func initClientset() bool {
	if podNamespace == "" || podDeployment == "" {
		return false
	}
	config, err := rest.InClusterConfig()
	if err != nil {
		return false
	}
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		clientset = nil
		return false
	}
	return true
}

var (
	podRE3 = regexp.MustCompile(`^([a-zA-Z0-9-]+)-([a-f0-9]{9,10})-([a-zA-Z0-9]{5})$`)
	podRE2 = regexp.MustCompile(`^([a-zA-Z0-9-]+)-([a-zA-Z0-9]{5})$`)
)

func parsePod(name string) (deployment, replicaSet, suffix string) {
	matches := podRE3.FindStringSubmatch(name)
	if len(matches) == 4 {
		return matches[1], matches[2], matches[3]
	}

	matches = podRE2.FindStringSubmatch(name)
	if len(matches) == 3 {
		return matches[1], "", matches[2]
	}

	return name, "", ""
}

func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func countPods(ctx context.Context) (count int, ok bool) {
	if clientset == nil {
		return 0, false
	}

	pods, err := clientset.CoreV1().Pods(podNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, false
	}

	podCount := 0
	for _, p := range pods.Items {
		otherDeployment, _, _ := parsePod(p.Name)
		if otherDeployment == podDeployment && isPodReady(&p) {
			podCount++
		}
	}

	return podCount, true
}

func refresh(ctx context.Context) {
	if count, ok := countPods(ctx); ok {
		podCount.Store(int32(count))
	} else {
		podCount.Store(-1)
	}
}

func runBackgroundRefresh(ctx context.Context) {
	if !initClientset() {
		zap.L().Warn("unable to count similar pods. disabling k8s functionality")
		podCount.Store(-1)
		return
	}

	refresh(ctx)
	go func() {
		time2.Sleep(ctx, jitter(podRefreshInterval))
		if ctx.Err() != nil {
			return
		}
		refresh(ctx)
	}()
}

// CountPods returns the number of pods like the current one.
func CountPods() (count int, ok bool) {
	initOnce.Do(func() {
		runBackgroundRefresh(context.Background())
	})

	count = int(podCount.Load())
	return count, count > 0
}
