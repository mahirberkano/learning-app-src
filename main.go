package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"math"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

//go:embed static
var staticFiles embed.FS

var (
	version = "0.2.0"

	httpRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests by endpoint and status",
		},
		[]string{"endpoint", "status"},
	)

	httpDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"endpoint"},
	)

	redisClient *redis.Client
)

func init() {
	prometheus.MustRegister(httpRequests, httpDuration)
}

func main() {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	staticFS, _ := fs.Sub(staticFiles, "static")
	http.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(http.FS(staticFS))))
	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/healthz", handleHealthz)
	http.HandleFunc("/load", handleLoad)
	http.Handle("/metrics", promhttp.Handler())

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("starting server version=%s port=%s redis=%s", version, port, redisAddr)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		httpDuration.WithLabelValues("/").Observe(time.Since(start).Seconds())
	}()

	hostname, _ := os.Hostname()

	ctx := context.Background()
	redisStatus := "connected"
	if err := redisClient.Ping(ctx).Err(); err != nil {
		redisStatus = fmt.Sprintf("error: %v", err)
	}

	httpRequests.WithLabelValues("/", "200").Inc()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"version":"%s","hostname":"%s","redis":"%s","goversion":"%s"}`,
		version, hostname, redisStatus, runtime.Version())
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		httpRequests.WithLabelValues("/healthz", "503").Inc()
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, `{"status":"unhealthy","redis":"%v"}`, err)
		return
	}

	httpRequests.WithLabelValues("/healthz", "200").Inc()
	fmt.Fprint(w, `{"status":"healthy"}`)
}

func handleLoad(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		httpDuration.WithLabelValues("/load").Observe(time.Since(start).Seconds())
	}()

	durationStr := r.URL.Query().Get("duration")
	if durationStr == "" {
		durationStr = "5s"
	}
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		http.Error(w, "invalid duration parameter", http.StatusBadRequest)
		return
	}
	if duration > 60*time.Second {
		http.Error(w, "max duration is 60s", http.StatusBadRequest)
		return
	}

	cores := runtime.NumCPU()

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	for i := 0; i < cores; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					math.Sqrt(float64(time.Now().UnixNano()))
				}
			}
		}()
	}

	wg.Wait()

	elapsed := time.Since(start)
	httpRequests.WithLabelValues("/load", "200").Inc()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"burned_cores":%d,"duration":"%s","actual":"%s"}`,
		cores, duration, elapsed.Round(time.Millisecond))
}
