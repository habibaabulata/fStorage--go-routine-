package handlers

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/habibaabulata/file_storage/storage"
)

func DownloadFile(c *gin.Context) {
	fileID := c.Param("fileID")
	log.Printf("Received request for fileID: %s", fileID)

	metadataBytes, err := storage.DB.Get([]byte(fileID), nil)
	if err != nil {
		log.Printf("Error retrieving metadata from database for fileID %s: %v", fileID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}
	log.Printf("Successfully retrieved metadata for file ID: %s", fileID)

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		log.Printf("Error unmarshalling metadata: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process file metadata"})
		return
	}

	chunkCount, ok := metadata["chunkCount"].(float64)
	if !ok {
		log.Println("Invalid chunkCount in metadata")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid file metadata"})
		return
	}

	var wg sync.WaitGroup
	chunkResults := make(chan []byte, int(chunkCount))
	errors := make(chan error, 1)

	// Helper function to read chunk
	readChunk := func(chunkIndex int) {
		defer wg.Done()

		chunkPath := filepath.Join("uploads", fileID+"_"+strconv.Itoa(chunkIndex))
		log.Printf("Reading chunk from path: %s", chunkPath)
		chunkContent, err := ioutil.ReadFile(chunkPath)
		if err != nil {
			log.Printf("Error reading file chunk %d: %v", chunkIndex, err)
			errors <- err
			return
		}
		chunkResults <- chunkContent
	}

	// Start chunk reading concurrently
	for i := 0; i < int(chunkCount); i++ {
		wg.Add(1)
		go readChunk(i)
	}

	// Close channels when done
	go func() {
		wg.Wait()
		close(chunkResults)
		close(errors)
	}()

	var fileContent []byte
	for {
		select {
		case err := <-errors:
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to download file chunks"})
				return
			}
		case chunk, ok := <-chunkResults:
			if !ok {
				log.Printf("Sending file with size: %d bytes", len(fileContent))
				c.Header("Content-Disposition", "attachment; filename="+metadata["filename"].(string))
				c.Data(http.StatusOK, "application/octet-stream", fileContent)
				return
			}
			fileContent = append(fileContent, chunk...)
		}
	}
}
