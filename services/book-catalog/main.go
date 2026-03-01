package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Book struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Author string `json:"author"`
	Status string `json:"status"`
}

var (
	bookDB   = make(map[string]Book)
	dbMu     sync.RWMutex
	dataFile = "books.json"
)

// File Storage Helpers

func loadData() {
	file, err := os.ReadFile(dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("books.json not found. Starting with an empty catalog.")
			return
		}
		log.Fatalf("Error reading data file: %v", err)
	}

	err = json.Unmarshal(file, &bookDB)
	if err != nil {
		log.Fatalf("Error parsing JSON data: %v", err)
	}
	log.Println("Loaded books from books.json")
}

func saveData() {
	data, err := json.MarshalIndent(bookDB, "", "  ")
	if err != nil {
		log.Printf("Error converting data to JSON: %v", err)
		return
	}

	err = os.WriteFile(dataFile, data, 0644)
	if err != nil {
		log.Printf("Error writing to data file: %v", err)
	}
}

func generateNextID() string {
	maxID := 0
	for idStr := range bookDB {
		idInt, err := strconv.Atoi(idStr)
		if err == nil && idInt > maxID {
			maxID = idInt
		}
	}
	return strconv.Itoa(maxID + 1)
}

// RabbitMQ Consumer

func setupRabbitMQConsumer() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Println("Warning: Could not connect to RabbitMQ. Status updates won't work automatically.")
		return
	}

	ch, err := conn.Channel()
	if err != nil {
		log.Println("Failed to open a channel")
		return
	}

	q, err := ch.QueueDeclare("book_status_queue", true, false, false, false, nil)
	if err != nil {
		log.Println("Failed to declare queue")
		return
	}

	err = ch.QueueBind(q.Name, "borrow.created", "borrow_events", false, nil)
	if err != nil {
		log.Println("Failed to bind queue")
		return
	}

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		log.Println("Failed to register consumer")
		return
	}

	log.Println("Successfully connected to RabbitMQ. Listening for borrow events...")

	go func() {
		for d := range msgs {
			var event struct {
				ID string `json:"book_id"`
			}
			if err := json.Unmarshal(d.Body, &event); err != nil {
				log.Printf("Error parsing event: %v", err)
				continue
			}

			dbMu.Lock()
			if book, exists := bookDB[event.ID]; exists {
				book.Status = "borrowed"
				bookDB[event.ID] = book
				saveData()
				log.Printf("RabbitMQ Event: Updated book '%s' status to 'borrowed'", event.ID)
			}
			dbMu.Unlock()
		}
	}()
}

// Main Application

func main() {
	loadData()

	setupRabbitMQConsumer()

	r := gin.Default()

	// Add Book
	r.POST("/books", func(c *gin.Context) {
		var newBook Book
		if err := c.ShouldBindJSON(&newBook); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
			return
		}

		if newBook.Status == "" {
			newBook.Status = "available"
		}

		dbMu.Lock()
		newBook.ID = generateNextID()
		bookDB[newBook.ID] = newBook
		saveData()
		dbMu.Unlock()

		c.JSON(http.StatusCreated, gin.H{"message": "Book added successfully", "book": newBook})
	})

	// View all books
	r.GET("/books", func(c *gin.Context) {
		dbMu.RLock()
		defer dbMu.RUnlock()

		var books []Book
		for _, book := range bookDB {
			books = append(books, book)
		}
		if books == nil {
			books = []Book{}
		}
		c.JSON(http.StatusOK, books)
	})

	// Edit book
	r.PUT("/books/:id", func(c *gin.Context) {
		id := c.Param("id")
		var updatedBook Book
		if err := c.ShouldBindJSON(&updatedBook); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
			return
		}

		dbMu.Lock()
		defer dbMu.Unlock()

		existingBook, exists := bookDB[id]
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
			return
		}

		if updatedBook.Title != "" {
			existingBook.Title = updatedBook.Title
		}
		if updatedBook.Author != "" {
			existingBook.Author = updatedBook.Author
		}
		if updatedBook.Status != "" {
			existingBook.Status = updatedBook.Status
		}

		bookDB[id] = existingBook
		saveData()

		c.JSON(http.StatusOK, gin.H{"message": "Book updated successfully", "book": existingBook})
	})

	// Delete book
	r.DELETE("/books/:id", func(c *gin.Context) {
		id := c.Param("id")

		dbMu.Lock()
		defer dbMu.Unlock()

		if _, exists := bookDB[id]; !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
			return
		}

		delete(bookDB, id)
		saveData()

		c.JSON(http.StatusOK, gin.H{"message": "Book deleted successfully"})
	})

	// check book status
	r.GET("/books/:id/status", func(c *gin.Context) {
		id := c.Param("id")

		dbMu.RLock()
		defer dbMu.RUnlock()

		book, exists := bookDB[id]
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": book.Status})
	})

	// search books
	r.GET("/books/search", func(c *gin.Context) {
		query := strings.ToLower(c.Query("q"))
		if query == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Search query 'q' is required"})
			return
		}

		var results []Book
		dbMu.RLock()
		defer dbMu.RUnlock()

		for _, book := range bookDB {
			if strings.Contains(strings.ToLower(book.Title), query) ||
				strings.Contains(strings.ToLower(book.Author), query) {
				results = append(results, book)
			}
		}
		if results == nil {
			results = []Book{}
		}
		c.JSON(http.StatusOK, results)
	})

	log.Println("Book Catalog Service running on port 8082...")
	r.Run(":8082")
}