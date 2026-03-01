package main

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// User struct สำหรับเก็บข้อมูลผู้ใช้
type User struct {
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	Status    string `json:"status"` // ACTIVE, SUSPENDED
	CreatedAt string `json:"created_at"`
}

// สำหรับรับข้อมูลที่ต้องการเก็บ ไม่ต้องเปลี่ยนทุกตัว
type UpdateUserInput struct {
	Name     *string `json:"name"`
	Email    *string `json:"email"`
	Password *string `json:"password"`
	Status   *string `json:"status"`
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

func main() {
	r := gin.Default()
	api := r.Group("/user")
	// Grouping routes เพื่อความเป็นระเบียบ (http://localhost:8082/user/...)
	{
		api.GET("", GetUsers)          // ดูทั้งหมด
		api.GET("/:id", GetUserByID)   // ดูรายคน
		api.POST("", CreateUser)       // เพิ่มคนใหม่
		api.PATCH("/:id", UpdateUser)  // แก้ไขข้อมูล
		api.DELETE("/:id", DeleteUser) // ลบข้อมูล
	}

	r.Run(":8082")
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

	// กำหนดค่าอัตโนมัติ
	newUser.Status = "ACTIVE"
	newUser.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	users, err := readUsers() // อ่านข้อมูลผู้ใช้ทั้งหมด
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read users"})
		return
	}
	users = append(users, newUser)
	err = writeUsers(users)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write users"})
		return
	}
	c.JSON(http.StatusCreated, newUser)
}

// PATCH /user/:id
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
				users[i].Email = *input.Email
			}

			if input.Status != nil {
				users[i].Status = *input.Status
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
