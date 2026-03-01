package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	dbmodel "borrow-return/db"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type BorrowEvent struct {
	BorrowID  string    `json:"borrow_id"`
	UserID    string    `json:"user_id"`
	BookID    string    `json:"book_id"`
	CreatedAt time.Time `json:"created_at"`
	DueDate   time.Time `json:"due_date"`
}

func isBookAvailable(bookID string) bool {
	url := "xxx" + bookID + "/status"

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

	return result.Status == "available"
}

func main() {
	// connect mongoDB
	collection, err := connectMongo()
	if err != nil {
		log.Fatal(err)
	}

	// connect rabbitMQ
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
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

		// check availability
		if !isBookAvailable(req.BookID) {
			c.JSON(400, gin.H{"error": "หนังสือเล่มนี้ถูกยืมไปแล้ว หรือไม่พร้อมให้บริการ"})
			return
		}

		// business logic
		borrowDays := 7
		now := time.Now()
		dueDate := now.AddDate(0, 0, borrowDays)

		borrowID := uuid.New().String()

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

	log.Println("Borrow Service is running on port 8081...")
	r.Run(":8081")
}
