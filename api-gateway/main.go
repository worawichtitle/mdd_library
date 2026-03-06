package main

import (
	"log"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
)

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
