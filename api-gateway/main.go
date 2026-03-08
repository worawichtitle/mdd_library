package main

import (
	"fmt"
	"log"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// นับจำนวน request ทั้งหมด
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_gateway_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// จับเวลา
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_gateway_request_duration_seconds",
			Help:    "Response time duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)

// register metrics
func init() {
	// register metrics with Prometheus
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(requestDuration)
}

func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		// เก็บ stat
		duration := time.Since(start).Seconds()
		status := fmt.Sprintf("%d", c.Writer.Status())
		method := c.Request.Method

		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}

		// บันทึก stat
		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		requestDuration.WithLabelValues(method, path).Observe(duration)
	}
}

const (
	BorrowServiceURL  = "http://borrow-return:8081"
	CatalogServiceURL = "http://book-catalog:8082"
	UserServiceURL    = "http://user-management:8083"
)

// Reverse Proxy
func reverseProxy(target string) gin.HandlerFunc {
	targetURL, _ := url.Parse(target)
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	return func(c *gin.Context) {
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

func main() {
	r := gin.Default()
	r.Use(PrometheusMiddleware())

	// metrics
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	r.Any("/user", reverseProxy(UserServiceURL))
	r.Any("/user/*filepath", reverseProxy(UserServiceURL))

	r.Any("/books", reverseProxy(CatalogServiceURL))
	r.Any("/books/*filepath", reverseProxy(CatalogServiceURL))

	r.Any("/copies", reverseProxy(CatalogServiceURL))
	r.Any("/copies/*any", reverseProxy(CatalogServiceURL))

	r.Any("/borrows", reverseProxy(BorrowServiceURL))
	r.Any("/borrows/*filepath", reverseProxy(BorrowServiceURL))

	log.Println("API Gateway is running on port 8000...")
	r.Run(":8000")
}
