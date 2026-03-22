package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/consul/api"
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

func discoverServiceURL(serviceName string) (string, error) {
	config := api.DefaultConfig()
	config.Address = "consul:8500"
	client, err := api.NewClient(config)
	if err != nil {
		return "", err
	}

	// ค้นหา service จากชื่อ
	services, _, err := client.Catalog().Service(serviceName, "", nil)
	if err != nil {
		return "", err
	}

	if len(services) == 0 {
		return "", fmt.Errorf("service '%s' not found in Consul", serviceName)
	}

	service := services[0]
	url := fmt.Sprintf("http://%s:%d", service.ServiceAddress, service.ServicePort)
	return url, nil
}


// Reverse Proxy
func reverseProxy(serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// ถาม consul
		targetURLStr, err := discoverServiceURL(serviceName)
		if err != nil {
			log.Printf("Service Discovery Error [%s]: %v", serviceName, err)
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": fmt.Sprintf("ระบบ %s ไม่พร้อมให้บริการในขณะนี้", serviceName),
			})
			return
		}

		targetURL, _ := url.Parse(targetURLStr)
		proxy := httputil.NewSingleHostReverseProxy(targetURL)

		// อัปเดต header
		c.Request.URL.Host = targetURL.Host
		c.Request.URL.Scheme = targetURL.Scheme
		c.Request.Header.Set("X-Forwarded-Host", c.Request.Header.Get("Host"))
		c.Request.Host = targetURL.Host

		// forward rquest
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
func main() {
	r := gin.Default()
	r.Use(PrometheusMiddleware())

	// metrics
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	r.Any("/user", reverseProxy("user-management-service"))
	r.Any("/user/*filepath", reverseProxy("user-management-service"))

	r.Any("/books", reverseProxy("book-catalog-service"))
	r.Any("/books/*filepath", reverseProxy("book-catalog-service"))

	r.Any("/copies", reverseProxy("book-catalog-service"))
	r.Any("/copies/*any", reverseProxy("book-catalog-service"))

	r.Any("/borrows", reverseProxy("borrow-return-service"))
	r.Any("/borrows/*filepath", reverseProxy("borrow-return-service"))

	log.Println("API Gateway is running on port 8000...")
	r.Run(":8000")
}
