package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	// "strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/consul/api"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Book struct {
	ISBN   string `json:"isbn"`
	Title  string `json:"title" binding:"required"`
	Author string `json:"author" binding:"required"`
}

type BookCopy struct {
	Barcode   string `json:"barcode"`
	ISBN      string `json:"isbn"`      // ชี้กลับไปหา Book
	Status    string `json:"status"`    // available, borrowed, lost, maintanance
	Condition string `json:"condition"` // new, good, damaged
}

type BookResponse struct {
	Book
	AvailableStock int `json:"available_stock"`
	TotalStock     int `json:"total_stock"`
}

// service discovery - register
func registerWithConsul() {
	config := api.DefaultConfig()
	config.Address = "consul:8500"
	client, err := api.NewClient(config)
	if err != nil {
		log.Fatalf("Failed to connect to Consul: %v", err)
	}

	registration := &api.AgentServiceRegistration{
		ID:      "book-catalog-service-1",
		Name:    "book-catalog-service",
		Port:    8082,
		Address: "book-catalog",
	}

	err = client.Agent().ServiceRegister(registration)
	if err != nil {
		log.Fatalf("Failed to register service: %v", err)
	}
	log.Println("Successfully registered with Consul Service Discovery")
}

var (
	booksDB    = make(map[string]Book)
	copiesDB   = make(map[string]BookCopy)
	dbMu       sync.RWMutex
	booksFile  = "books.json"
	copiesFile = "copies.json"
)

// File Storage Helpers

func loadData() {
	// load Books
	file, err := os.ReadFile(booksFile)
	if err == nil {
		json.Unmarshal(file, &booksDB)
	} else if os.IsNotExist(err) {
		log.Println("books.json not found. Starting fresh.")
	}

	// load book copies
	file2, err := os.ReadFile(copiesFile)
	if err == nil {
		json.Unmarshal(file2, &copiesDB)
	} else if os.IsNotExist(err) {
		log.Println("copies.json not found. Starting fresh.")
	}
	log.Println("Loaded data successfully.")
}

func saveData() {
	// Save Books
	booksData, err := json.MarshalIndent(booksDB, "", "  ")
	if err == nil {
		os.WriteFile(booksFile, booksData, 0644)
	} else {
		log.Printf("Error saving books: %v", err)
	}

	// Save Copies
	copiesData, err := json.MarshalIndent(copiesDB, "", "  ")
	if err == nil {
		os.WriteFile(copiesFile, copiesData, 0644)
	} else {
		log.Printf("Error saving copies: %v", err)
	}
}

func generateISBN() string {
	maxID := 0
	for idStr := range booksDB {
		var current int
		fmt.Sscanf(idStr, "ISBN-%d", &current)
		if current > maxID {
			maxID = current
		}
	}
	return fmt.Sprintf("ISBN-%d", maxID+1)
}

func generateBarcode() string {
	maxID := 0
	for idStr := range copiesDB {
		var current int
		fmt.Sscanf(idStr, "BC-%d", &current)
		if current > maxID {
			maxID = current
		}
	}
	return fmt.Sprintf("BC-%d", maxID+1)
}

// func generateNextID() string {
// 	maxID := 0
// 	for idStr := range bookDB {
// 		idInt, err := strconv.Atoi(idStr)
// 		if err == nil && idInt > maxID {
// 			maxID = idInt
// 		}
// 	}
// 	return strconv.Itoa(maxID + 1)
// }

// RabbitMQ Consumer

func setupRabbitMQConsumer() {
	var conn *amqp.Connection
	var err error

	for i := 0; i < 10; i++ {
		conn, err = amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
		if err == nil {
			log.Println("Catalog Service successfully connected to RabbitMQ!")
			break
		}

		log.Printf("Catalog trying to connect RabbitMQ (attempt %d/10)...", i+1)
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		log.Printf("Failed to connect to RabbitMQ after retries: %v", err)
		return
	}

	ch, err := conn.Channel()
	if err != nil {
		log.Printf("RabbitMQ Channel Error: %v", err)
		return
	}
	err = ch.ExchangeDeclare(
		"borrow_events",
		"topic",
		true, false, false, false, nil,
	)
	if err != nil {
		log.Printf("Exchange Declare Error: %v", err)
		return
	}
	q, err := ch.QueueDeclare("book_status_queue", true, false, false, false, nil)
	if err != nil {
		log.Printf("Queue Declare Error: %v", err)
		return
	}

	// 1. Listen for Borrows
	ch.QueueBind(q.Name, "borrow.created", "borrow_events", false, nil)

	// 2. Listen for Returns
	ch.QueueBind(q.Name, "borrow.returned", "borrow_events", false, nil)

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		log.Println("Failed to register consumer")
		return
	}

	log.Println("RabbitMQ Connected. Listening for borrow and return events...")

	go func() {
		for d := range msgs {
			var event struct {
				Barcode string `json:"barcode"`
			}
			if err := json.Unmarshal(d.Body, &event); err != nil {
				continue
			}

			dbMu.Lock()
			if copy, exists := copiesDB[event.Barcode]; exists {

				// Check which event we just received from RabbitMQ
				if d.RoutingKey == "borrow.created" {
					if copy.Status == "available" {
						copy.Status = "borrowed"
						log.Printf("RabbitMQ Event: Barcode '%s' borrowed", event.Barcode)
					} else {
						log.Printf("RabbitMQ Warning: Barcode '%s' is already %s!", event.Barcode, copy.Status)
					}
				} else if d.RoutingKey == "borrow.returned" {
					// If it's a return event, make the book available again
					copy.Status = "available"
					log.Printf("RabbitMQ Event: Barcode '%s' returned and is now available", event.Barcode)
				}

				// Save the updated copy back to the database
				copiesDB[event.Barcode] = copy
				saveData()

			} else {
				log.Printf("RabbitMQ Error: Barcode '%s' not found in inventory", event.Barcode)
			}
			dbMu.Unlock()
		}
	}()
}

// Main Application

func main() {
	// register consul
	registerWithConsul()

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

		dbMu.Lock()
		newBook.ISBN = generateISBN()
		booksDB[newBook.ISBN] = newBook
		saveData()
		dbMu.Unlock()

		c.JSON(http.StatusCreated, gin.H{"message": "Book catalog added", "book": newBook})
	})

	// View all books
	r.GET("/books", func(c *gin.Context) {
		dbMu.RLock()
		defer dbMu.RUnlock()

		var results []BookResponse
		for _, book := range booksDB {
			available := 0
			total := 0

			// นับ stock สดๆ จากตาราง Copies
			for _, copy := range copiesDB {
				if copy.ISBN == book.ISBN {
					total++
					if copy.Status == "available" {
						available++
					}
				}
			}

			results = append(results, BookResponse{
				Book:           book,
				AvailableStock: available,
				TotalStock:     total,
			})
		}
		c.JSON(http.StatusOK, results)
	})

	// get copies of a specificbook
	r.GET("/books/:isbn/copies", func(c *gin.Context) {
		isbn := c.Param("isbn")

		dbMu.RLock()
		defer dbMu.RUnlock()

		var copies []BookCopy
		for _, copy := range copiesDB {
			if copy.ISBN == isbn {
				copies = append(copies, copy)
			}
		}

		c.JSON(http.StatusOK, copies)
	})

	// add physical copy of book
	r.POST("/copies", func(c *gin.Context) {
		var newCopy BookCopy

		if err := c.ShouldBindJSON(&newCopy); err != nil || newCopy.ISBN == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input, ISBN required to link copy to a book"})
			return
		}

		dbMu.Lock()
		defer dbMu.Unlock()

		// เช็คว่ามีข้อมูลหนังสือในระบบไหม
		if _, exists := booksDB[newCopy.ISBN]; !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ISBN not found in catalog. Add book first."})
			return
		}

		if newCopy.Status == "" {
			newCopy.Status = "available"
		}
		if newCopy.Condition == "" {
			newCopy.Condition = "new"
		}

		newCopy.Barcode = generateBarcode()
		copiesDB[newCopy.Barcode] = newCopy
		saveData()

		c.JSON(http.StatusCreated, gin.H{"message": "Physical copy added", "copy": newCopy})
	})

	// edit book catalog (Title/Author only)
	r.PUT("/books/:isbn", func(c *gin.Context) {
		isbn := c.Param("isbn")

		var updateData map[string]interface{}
		if err := c.ShouldBindJSON(&updateData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
			return
		}

		dbMu.Lock()
		defer dbMu.Unlock()

		existingBook, exists := booksDB[isbn]
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
			return
		}

		if title, ok := updateData["title"].(string); ok {
			existingBook.Title = title
		}
		if author, ok := updateData["author"].(string); ok {
			existingBook.Author = author
		}

		booksDB[isbn] = existingBook
		saveData()

		c.JSON(http.StatusOK, gin.H{"message": "Book updated successfully", "book": existingBook})
	})

	// delete book catalog (cant delete if copies still exist)
	r.DELETE("/books/:isbn", func(c *gin.Context) {
		isbn := c.Param("isbn")

		dbMu.Lock()
		defer dbMu.Unlock()

		// check if book exists
		if _, exists := booksDB[isbn]; !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Book catalog not found"})
			return
		}

		// check if there are still physical copies attached to this ISBN
		for _, copy := range copiesDB {
			if copy.ISBN == isbn {
				c.JSON(http.StatusConflict, gin.H{
					"error": "Cannot delete book catalog. Physical copies still exist in inventory.",
				})
				return
			}
		}

		// if no copies exist, delete
		delete(booksDB, isbn)
		saveData()

		c.JSON(http.StatusOK, gin.H{"message": "Book catalog deleted successfully"})
	})

	// // Delete book
	// r.DELETE("/books/:id", func(c *gin.Context) {
	// 	id := c.Param("id")

	// 	dbMu.Lock()
	// 	defer dbMu.Unlock()

	// 	if _, exists := bookDB[id]; !exists {
	// 		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
	// 		return
	// 	}

	// 	delete(bookDB, id)
	// 	saveData()

	// 	c.JSON(http.StatusOK, gin.H{"message": "Book deleted successfully"})
	// })

	// // check book status
	// r.GET("/books/:id/status", func(c *gin.Context) {
	// 	id := c.Param("id")

	// 	dbMu.RLock()
	// 	defer dbMu.RUnlock()

	// 	book, exists := bookDB[id]
	// 	if !exists {
	// 		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
	// 		return
	// 	}

	// 	c.JSON(http.StatusOK, gin.H{"status": book.Status})
	// })

	// search books
	r.GET("/books/search", func(c *gin.Context) {
		query := strings.ToLower(c.Query("q"))
		if query == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Search query 'q' is required"})
			return
		}

		dbMu.RLock()
		defer dbMu.RUnlock()

		var results []BookResponse
		for _, book := range booksDB {
			if strings.Contains(strings.ToLower(book.Title), query) ||
				strings.Contains(strings.ToLower(book.Author), query) {

				// นับ stock สำหรับเล่มที่ค้นเจอ
				available := 0
				total := 0
				for _, copy := range copiesDB {
					if copy.ISBN == book.ISBN {
						total++
						if copy.Status == "available" {
							available++
						}
					}
				}

				results = append(results, BookResponse{
					Book:           book,
					AvailableStock: available,
					TotalStock:     total,
				})
			}
		}
		c.JSON(http.StatusOK, results)
	})

	// update book copy status/condition
	r.PUT("/copies/:barcode", func(c *gin.Context) {
		barcode := c.Param("barcode")

		var updateData map[string]string
		if err := c.ShouldBindJSON(&updateData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
			return
		}

		dbMu.Lock()
		defer dbMu.Unlock()

		copy, exists := copiesDB[barcode]
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Copy not found"})
			return
		}

		if status, ok := updateData["status"]; ok {
			copy.Status = status
		}
		if condition, ok := updateData["condition"]; ok {
			copy.Condition = condition
		}

		copiesDB[barcode] = copy
		saveData()

		c.JSON(http.StatusOK, gin.H{"message": "Copy updated successfully", "copy": copy})
	})

	// check status of specific copy
	r.GET("/copies/:barcode/status", func(c *gin.Context) {
		barcode := c.Param("barcode")

		dbMu.RLock()
		defer dbMu.RUnlock()

		copy, exists := copiesDB[barcode]
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Copy not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": copy.Status})
	})

	// delete a specific physical copy
	r.DELETE("/copies/:barcode", func(c *gin.Context) {
		barcode := c.Param("barcode")

		dbMu.Lock()
		defer dbMu.Unlock()

		// check if the physical copy exists
		if _, exists := copiesDB[barcode]; !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Physical copy not found"})
			return
		}

		// delete the copy
		delete(copiesDB, barcode)
		saveData()

		c.JSON(http.StatusOK, gin.H{"message": "Physical copy deleted successfully"})
	})

	log.Println("Book Catalog Service running on port 8082...")
	r.Run(":8082")
}
