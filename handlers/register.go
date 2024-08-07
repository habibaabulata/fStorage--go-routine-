package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"

	// "sync"

	"github.com/gin-gonic/gin"
	// "github.com/syndtr/goleveldb/leveldb"
	"github.com/habibaabulata/file_storage/storage"
)

var registerChannel = make(chan string)

func Register(c *gin.Context) {
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&credentials); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	go func() {
		defer close(registerChannel)
		hash := sha256.New()
		hash.Write([]byte(credentials.Password))
		hashedPassword := hex.EncodeToString(hash.Sum(nil))

		err := storage.DB.Put([]byte(credentials.Username), []byte(hashedPassword), nil)
		if err != nil {
			log.Printf("Failed to register user %s: %v", credentials.Username, err)
			registerChannel <- "fail"
		} else {
			log.Printf("User %s registered successfully", credentials.Username)
			registerChannel <- "success"
		}
	}()

	result := <-registerChannel
	if result == "success" {
		c.JSON(http.StatusOK, gin.H{"message": "Registration successful"})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Registration failed"})
	}
}
