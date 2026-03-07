package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	amqp "github.com/rabbitmq/amqp091-go"
	"golang.org/x/crypto/bcrypt"
)

// User struct สำหรับเก็บข้อมูลผู้ใช้
type User struct {
	UserID    string `json:"user_id"`
	Name      string `json:"name" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required"`
	Status    string `json:"status"` // ACTIVE, SUSPENDED
	Role      string `json:"role"`   // GUEST, STUDENT, TEACHER, STAFF
	CreatedAt string `json:"created_at"`
}

// สำหรับรับข้อมูลที่ต้องการเก็บ ไม่ต้องเปลี่ยนทุกตัว
type UpdateUserInput struct {
	Name     *string `json:"name" binding:"omitempty"`
	Email    *string `json:"email" binding:"omitempty,email"`
	Password *string `json:"password" binding:"omitempty"`
	Role     *string `json:"role" binding:"omitempty"`
	Status   *string `json:"status" binding:"omitempty"`
}

var filePath = "db/user.json"

// อ่านข้อมูลจากไฟล์ user.json
func readUsers() ([]User, error) {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var users []User
	json.Unmarshal(file, &users)
	return users, nil
}

// เขียนข้อมูลลงไฟล์
func writeUsers(users []User) error {
	data, _ := json.MarshalIndent(users, "", "  ")
	return os.WriteFile(filePath, data, 0644)
}

func generateNextID(users []User) string {
	maxID := 0

	for _, u := range users {
		var id int
		fmt.Sscanf(u.UserID, "U%d", &id)
		if id > maxID {
			maxID = id
		}
	}
	return fmt.Sprintf("U%d", maxID+1)
}

func main() {
	// set up RabbitMQ
	conn, err := amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
	if err != nil {
		log.Printf("RabbitMQ not available, continuing without events: %v", err)
	} else {
		defer conn.Close()
		ch, err := conn.Channel()
		if err != nil {
			log.Printf("Failed to open channel: %v", err)
		} else {
			defer ch.Close()
			go listenToBorrowEvents(ch) // start consumer goroutine
		}
	}

	r := gin.Default()
	api := r.Group("/user")
	// Grouping routes เพื่อความเป็นระเบียบ (http://localhost:8083/user/...)
	{
		api.GET("", GetUsers)              // ดูทั้งหมด
		api.GET("/:id", GetUserByID)       // ดูรายคน
		api.GET("/:id/verify", VerifyUser) // ตรวจสอบผู้ใช้
		api.POST("", CreateUser)           // เพิ่มคนใหม่
		api.PUT("/:id", UpdateUser)        // แก้ไขข้อมูล
		api.DELETE("/:id", DeleteUser)     // ลบข้อมูล
	}

	r.Run(":8083")
}

// GET /user
func GetUsers(c *gin.Context) {
	users, err := readUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read users"})
		return
	}
	c.JSON(http.StatusOK, users)
}

// GET /user/:id
func GetUserByID(c *gin.Context) {
	id := c.Param("id")
	users, err := readUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read users"})
		return
	}
	for _, user := range users {
		if user.UserID == id {
			c.JSON(http.StatusOK, user)
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
}

// GET /user/:id/verify
func VerifyUser(c *gin.Context) {
	id := c.Param("id")
	users, err := readUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read users"})
		return
	}
	for _, user := range users {
		if user.UserID == id {
			if user.Status == "ACTIVE" {
				c.JSON(http.StatusOK, gin.H{"valid": true, "role": user.Role})
				return
			}
			c.JSON(http.StatusOK, gin.H{"valid": false})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
}

// POST /user
func CreateUser(c *gin.Context) {
	var newUser User                                   // รับข้อมูล user ใหม่
	if err := c.ShouldBindJSON(&newUser); err != nil { // ตรวจความถูกต้องข้อมูล
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// hash password
	hashed, err := bcrypt.GenerateFromPassword([]byte(newUser.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}
	newUser.Password = string(hashed)

	users, err := readUsers() // อ่านข้อมูลผู้ใช้ทั้งหมด
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read users"})
		return
	}
	// ตรวจ email ซ้ำ
	for _, user := range users {
		if user.Email == newUser.Email {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email already exists"})
			return
		}
	}

	// กำหนดค่าอัตโนมัติ
	newUser.UserID = generateNextID(users)
	newUser.Status = "ACTIVE"
	newUser.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	if newUser.Role == "" {
		newUser.Role = "GUEST"
	} else {
		newUser.Role = strings.ToUpper(newUser.Role)
	}

	// เพิ่มผู้ใช้
	users = append(users, newUser)
	err = writeUsers(users)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write users"})
		return
	}
	c.JSON(http.StatusCreated, newUser)
}

// PUT /user/:id
func UpdateUser(c *gin.Context) {
	id := c.Param("id")
	var input UpdateUserInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	users, err := readUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read users"})
		return
	}

	for i, user := range users {
		if user.UserID == id {

			if input.Name != nil {
				users[i].Name = *input.Name
			}

			if input.Email != nil {
				for _, user := range users {
					if user.Email == *input.Email {
						c.JSON(http.StatusBadRequest, gin.H{"error": "Email already exists"})
						return
					}
				}
				users[i].Email = *input.Email
			}

			if input.Status != nil {
				users[i].Status = strings.ToUpper(*input.Status)
			}

			if input.Role != nil {
				users[i].Role = strings.ToUpper(*input.Role)
			}

			if input.Password != nil {
				hashed, err := bcrypt.GenerateFromPassword([]byte(*input.Password), bcrypt.DefaultCost)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
					return
				}
				users[i].Password = string(hashed)
			}

			if err := writeUsers(users); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
				return
			}

			c.JSON(http.StatusOK, users[i])
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
}

// DELETE /user/:id
func DeleteUser(c *gin.Context) {
	id := c.Param("id")
	users, err := readUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read users"})
		return
	}
	for i, user := range users {
		if user.UserID == id {
			users = append(users[:i], users[i+1:]...) // ลบผู้ใช้จาก slice
			err = writeUsers(users)
			c.JSON(http.StatusOK, gin.H{"message": "User deleted"})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
}

// สำหรับฟัง event การยืมหนังสือจาก RabbitMQ
func listenToBorrowEvents(ch *amqp.Channel) {
	q, err := ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		log.Printf("queue declare error: %v", err)
		return
	}
	if err = ch.QueueBind(q.Name, "borrow.*", "borrow_events", false, nil); err != nil {
		log.Printf("queue bind error: %v", err)
		return
	}
	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		log.Printf("consume error: %v", err)
		return
	}
	for msg := range msgs {
		log.Printf("borrow event received: %s", msg.Body)
		// TODO: update user record if needed
	}
}
