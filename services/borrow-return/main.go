package main

import (
	dbmodel "borrow-return/db"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	gonanoid "github.com/matoous/go-nanoid/v2"
	amqp "github.com/rabbitmq/amqp091-go"
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

func isBookAvailable(bookID string) bool {
	url := fmt.Sprintf("http://book-catalog:8082/books/%s/status", bookID)

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil || resp.StatusCode != 200 {
		return false
	}
	defer resp.Body.Close()

	var result struct {
		Status string `json:"status"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	return strings.TrimSpace(strings.ToLower(result.Status)) == "available"
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
	r.POST("/borrows", func(c *gin.Context) {
		var req struct {
			UserID string `json:"user_id" binding:"required"`
			BookID string `json:"book_id" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// check book availability
		// if !isBookAvailable(req.BookID) {
		// 	c.JSON(400, gin.H{"error": "หนังสือเล่มนี้ถูกยืมไปแล้ว หรือไม่พร้อมให้บริการ"})
		// 	return
		// }

		// business logic
		borrowDays := 7
		now := time.Now()
		dueDate := now.AddDate(0, 0, borrowDays)

		id, _ := gonanoid.New(8)
		borrowID := fmt.Sprintf("BRW-%s", id)

		// db
		newBorrow := dbmodel.Borrow{
			BorrowID:   borrowID,
			UserID:     req.UserID,
			BookID:     req.BookID,
			BorrowDate: now,
			DueDate:    dueDate,
			Status:     "BORROWED",
			FineAmount: 0,
		}
		ctxDB, cancelDB := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelDB()

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

	r.GET("/borrows", func(c *gin.Context) {
		// query
		userID := c.Query("user_id")
		status := c.Query("status")

		filter := bson.M{}
		if userID != "" {
			filter["user_id"] = userID
		}
		if status != "" {
			filter["status"] = status
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
