package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/habibaabulata/file_storage/storage"
)

var jwtKey = []byte("JWT_SECRET_KEY")

type Claims struct {
	Username string `json:"username"`
	jwt.StandardClaims
}

// GenerateToken generates a new JWT token
func GenerateToken(username string) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Username: username,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

// Authenticate middleware to validate JWT token
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Remove "Bearer " prefix from token string if it exists
		if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			tokenString = tokenString[7:]
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})

		if err != nil {
			if err == jwt.ErrSignatureInvalid {
				log.Printf("Invalid token signature: %v", err)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token signature"})
				c.Abort()
				return
			}
			log.Printf("Token parsing error: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		if !token.Valid {
			log.Println("Token is invalid")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		log.Printf("Authenticated request by user: %s", claims.Username)
		c.Set("username", claims.Username)
		c.Next()
	}
}

// Login handler for generating a token
var loginChannel = make(chan string)
var wg sync.WaitGroup

func Login(c *gin.Context) {
	wg.Add(1)
	defer wg.Wait()

	go func() {
		defer wg.Done()

		var credentials struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&credentials); err != nil {
			loginChannel <- "invalid"
			return
		}

		// Fetch hashed password from the database
		storedPassword, err := storage.DB.Get([]byte(credentials.Username), nil)
		if err != nil {
			loginChannel <- "unauthorized"
			return
		}

		// Hash the provided password
		hash := sha256.New()
		hash.Write([]byte(credentials.Password))
		hashedPassword := hex.EncodeToString(hash.Sum(nil))

		if hashedPassword != string(storedPassword) {
			loginChannel <- "unauthorized"
		} else {
			// Generate token on successful login
			token, err := GenerateToken(credentials.Username)
			if err != nil {
				loginChannel <- "error"
				return
			}
			loginChannel <- token
		}
	}()

	result := <-loginChannel
	switch result {
	case "unauthorized":
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
	case "error":
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
	case "invalid":
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	default:
		c.JSON(http.StatusOK, gin.H{"token": result})
	}
}
