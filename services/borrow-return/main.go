package main

import (
	dbmodel "borrow-return/db"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	gonanoid "github.com/matoous/go-nanoid/v2"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sony/gobreaker"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type BorrowEvent struct {
	BorrowID  string    `json:"borrow_id"`
	UserID    string    `json:"user_id"`
	BookID    string    `json:"book_id"`
	CreatedAt time.Time `json:"created_at"`
	DueDate   time.Time `json:"due_date"`
}

type ReturnEvent struct {
	BorrowID   string    `json:"borrow_id"`
	UserID     string    `json:"user_id"`
	BookID     string    `json:"book_id"`
	ReturnDate time.Time `json:"return_date"`
	DaysLate   int       `json:"days_late"`
}

type UserVerifyResponse struct {
	Valid bool   `json:"valid"`
	Role  string `json:"role"`
}

type BookStatusResponse struct {
	Status string `json:"status"`
}

var userCB *gobreaker.CircuitBreaker
var catalogCB *gobreaker.CircuitBreaker

// setting ciruit breaker
func init() {
	settings := gobreaker.Settings{
		MaxRequests: 1,                // Half-Open: ให้ผ่าน 1 request เพื่อ test
		Interval:    time.Minute,      // Reset counts ทุก 1 นาที
		Timeout:     15 * time.Second, // Open State นาน 15 วิแล้วเปลี่ยนเป็น Half-Open
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			// ถ้า Error เกิน 40% และ Request รวมเกิน 5 ครั้ง ให้ตัดวงจร (Open)
			return counts.Requests >= 5 && failureRatio >= 0.4
		},
	}

	// cb user management service
	settings.Name = "User-Service-CB"
	userCB = gobreaker.NewCircuitBreaker(settings)

	// cb book catalog service
	settings.Name = "Book-Service-CB"
	catalogCB = gobreaker.NewCircuitBreaker(settings)
}

// retry with circuit breaker
func callAPIWithBreaker(cb *gobreaker.CircuitBreaker, url string, serviceName string) ([]byte, error) {
	var finalErr error

	// Retry 3 ครั้ง
	for i := 0; i < 3; i++ {
		res, cbErr := cb.Execute(func() (interface{}, error) {
			// Setup Timeout
			client := &http.Client{Timeout: 2 * time.Second}
			resp, err := client.Get(url)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 500 {
				return nil, fmt.Errorf("server error: %d", resp.StatusCode)
			}
			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("not found or invalid status: %d", resp.StatusCode)
			}
			// อ่านข้อมูลที่ได้มาเป็น []byte แล้ว return กับ
			body, err := io.ReadAll(resp.Body)
			return body, err
		})

		// สำเร็จ
		if cbErr == nil {
			return res.([]byte), nil
		}

		// ถ้า Circuit Breaker Open อยู่มันจะ return error ทันที โดยที่ไม่ยิง request จริง
		if cbErr == gobreaker.ErrOpenState {
			log.Printf("%s Circuit is OPEN. Stopping retries.\n", serviceName)
			return nil, fmt.Errorf("ระบบ %s ไม่พร้อมใช้งานชั่วคราว (Circuit Open)", serviceName)
		}

		finalErr = cbErr
		log.Printf("%s attempt %d failed: %v. Retrying...\n", serviceName, i+1, cbErr)
		time.Sleep(1 * time.Second) // Simple Backoff
	}

	return nil, finalErr
}

// check book | call book catalog service
func isBookAvailable(bookID string) (bool, error) {
	var err error
	url := fmt.Sprintf("http://book-catalog:8082/books/%s/status", bookID)

	body, err := callAPIWithBreaker(catalogCB, url, "Book Catalog")
	if err != nil {
		return false, err
	}

	var result BookStatusResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return false, err
	}

	isAvailable := strings.TrimSpace(strings.ToLower(result.Status)) == "available"
	return isAvailable, nil
}

// verify user | call user management service
func verifyUser(userID string) (*UserVerifyResponse, error) {
	var err error
	url := fmt.Sprintf("http://user-management:8083/user/%s/verify", userID)

	body, err := callAPIWithBreaker(userCB, url, "User Management")
	if err != nil {
		return nil, err
	}

	var result UserVerifyResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func main() {
	// connect mongoDB
	collection, err := connectMongo()
	if err != nil {
		log.Fatal(err)
	}

	// connect rabbitMQ
	conn, err := amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}
	defer ch.Close()

	// exchange
	err = ch.ExchangeDeclare(
		"borrow_events", // name
		"topic",         // type
		true, false, false, false, nil,
	)

	r := gin.Default()

	// endpoint
	//borrow book
	r.POST("/borrows", func(c *gin.Context) {
		var req struct {
			UserID string `json:"user_id" binding:"required"`
			BookID string `json:"book_id" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// user verify
		user, err := verifyUser(req.UserID)
		if err != nil {
			log.Printf("err: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if !user.Valid {
			c.JSON(http.StatusForbidden, gin.H{"error": "บัญชีผู้ใช้ไม่สามารถทำการยืมได้ หรือถูกระงับสิทธิ์"})
			return
		}

		// borrow quota
		var maxBooks, borrowDays int
		switch strings.ToUpper(user.Role) {
		case "TEACHER", "STAFF":
			maxBooks = 10
			borrowDays = 30
		case "STUDENT":
			maxBooks = 5
			borrowDays = 14
		case "GUEST":
			maxBooks = 1
			borrowDays = 7
		default:
			maxBooks = 1
			borrowDays = 7
		}

		// check current borrow
		ctxDB, cancelDB := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelDB()

		borrowCount, err := collection.CountDocuments(ctxDB, bson.M{
			"user_id": req.UserID,
			"status":  "BORROWED",
		})
		if err != nil {
			log.Printf("err: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if int(borrowCount) >= maxBooks {
			c.JSON(http.StatusForbidden, gin.H{
				"error": fmt.Sprintf("คุณไม่สามารถยืมได้ เนื่องจากยืมหนังสือครบโควต้าแล้ว (%d เล่ม)", maxBooks),
			})
			return
		}

		// check book availability
		// if !isBookAvailable(req.BookID) {
		// 	c.JSON(400, gin.H{"error": "หนังสือเล่มนี้ถูกยืมไปแล้ว หรือไม่พร้อมให้บริการ"})
		// 	return
		// }

		// save db
		now := time.Now()
		dueDate := now.AddDate(0, 0, borrowDays)

		alphabet := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		id, _ := gonanoid.Generate(alphabet, 10)
		borrowID := fmt.Sprintf("BRW-%s", id)

		newBorrow := dbmodel.Borrow{
			BorrowID:   borrowID,
			UserID:     req.UserID,
			BookID:     req.BookID,
			BorrowDate: now,
			DueDate:    dueDate,
			Status:     "BORROWED",
			FineAmount: 0,
		}

		_, err = collection.InsertOne(ctxDB, newBorrow)
		if err != nil {
			log.Printf("Failed to insert borrow record: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "ระบบฐานข้อมูลมีปัญหา ไม่สามารถทำรายการได้"})
			return
		}

		// publish event
		event := BorrowEvent{
			BorrowID:  borrowID,
			UserID:    req.UserID,
			BookID:    req.BookID,
			CreatedAt: now,
			DueDate:   dueDate,
		}
		body, _ := json.Marshal(event)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = ch.PublishWithContext(ctx,
			"borrow_events",
			"borrow.created",
			false, false,
			amqp.Publishing{
				ContentType: "application/json",
				Body:        body,
			})

		if err != nil {
			log.Printf("Failed to publish event: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Borrow saved but failed to publish event"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message":   "ยืมหนังสือสำเร็จ",
			"borrow_id": borrowID,
			"due_date":  dueDate.Format("2006-01-02"),
		})
	})

	//return book
	r.POST("/borrows/:id/return", func(c *gin.Context) {
		borrowID := c.Param("id")

		ctxDB, cancelDB := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelDB()

		// lookup db
		var borrow dbmodel.Borrow

		err := collection.FindOne(ctxDB, bson.M{"borrow_id": borrowID, "status": "BORROWED"}).Decode(&borrow)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "ไม่พบข้อมูลการยืมนี้ หรือหนังสือถูกคืนไปแล้ว"})
			return
		}

		now := time.Now()
		daysLate := 0
		if now.After(borrow.DueDate) {
			duration := now.Sub(borrow.DueDate)
			daysLate = int(duration.Hours() / 24)
		}

		update := bson.M{
			"$set": bson.M{
				"status":      "RETURNED",
				"return_date": now,
				"days_late":   daysLate,
			},
		}

		_, err = collection.UpdateOne(ctxDB, bson.M{"borrow_id": borrowID}, update)
		if err != nil {
			log.Printf("Failed to update: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "อัปเดตไม่สำเร็จ"})
			return
		}

		// publish event
		event := ReturnEvent{
			BorrowID:   borrow.BorrowID,
			UserID:     borrow.UserID,
			BookID:     borrow.BookID,
			ReturnDate: now,
			DaysLate:   daysLate,
		}
		body, _ := json.Marshal(event)

		ctxMQ, cancelMQ := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelMQ()

		err = ch.PublishWithContext(ctxMQ,
			"borrow_events",
			"borrow.returned",
			false, false,
			amqp.Publishing{
				ContentType: "application/json",
				Body:        body,
			})

		if err != nil {
			log.Printf("Failed to publish return event: %v", err)
		}

		c.JSON(http.StatusOK, gin.H{
			"message":     "คืนหนังสือสำเร็จ",
			"borrow_id":   borrow.BorrowID,
			"return_date": now,
			"days_late":   daysLate,
		})
	})

	//get borrows
	r.GET("/borrows", func(c *gin.Context) {
		// query
		userID := c.Query("user_id")
		status := c.Query("status")
		isOverdue := c.Query("overdue")
		filter := bson.M{}
		if userID != "" {
			filter["user_id"] = userID
		}
		if status != "" {
			filter["status"] = status
		}
		if isOverdue == "true" || isOverdue == "false" {
			filter["status"] = "BORROWED"
			op := "$lt"
			if isOverdue == "false" {
				op = "$gte"
			}
			filter["due_date"] = bson.M{
				op: time.Now(),
			}
		}

		ctxDB, cancelDB := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelDB()

		// find
		cursor, err := collection.Find(ctxDB, filter)
		if err != nil {
			log.Printf("Failed to fetch borrows: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer cursor.Close(ctxDB)

		// convert to arr
		var borrows []dbmodel.Borrow
		if err = cursor.All(ctxDB, &borrows); err != nil {
			log.Printf("Failed to decode borrows: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if borrows == nil {
			borrows = []dbmodel.Borrow{}
		}

		c.JSON(http.StatusOK, borrows)
	})

	//get borrow by id
	r.GET("/borrows/:id", func(c *gin.Context) {
		// param
		borrowID := c.Param("id")

		ctxDB, cancelDB := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelDB()

		// find
		var borrow dbmodel.Borrow
		err := collection.FindOne(ctxDB, bson.M{"borrow_id": borrowID}).Decode(&borrow)

		if err != nil {
			// not found
			if err == mongo.ErrNoDocuments {
				c.JSON(http.StatusNotFound, gin.H{"error": "ไม่พบข้อมูลการยืม"})
				return
			}
			// db err
			log.Printf("Database error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, borrow)
	})

	log.Println("Borrow Service is running on port 8081...")
	r.Run(":8081")
}
