package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/golang/glog"

	"gopkg.in/gin-gonic/gin.v1"
)

var (
	pgDb, pgUser, pgPassword, pgHost string
	queueTopic, uploadFolderPath     string
)

func main() {
	loadEnvironmentVariables()
	CreateSchemas(pgUser, pgPassword, pgHost, pgDb)
	startAPIServer()
}

// loadEnvironmentVariables loads PostgreSQL
// information from dotenv
func loadEnvironmentVariables() {
	pgDb = os.Getenv("PGDB")
	if len(pgDb) == 0 {
		panic("No PGDB environment variable")
	}

	pgUser = os.Getenv("PGUSER")
	if len(pgUser) == 0 {
		panic("No PGUSER environment variable")
	}

	pgPassword = os.Getenv("PGPASSWORD")
	if len(pgPassword) == 0 {
		panic("No PGPASSWORD environment variable")
	}

	pgHost = os.Getenv("PGHOST")
	if len(pgHost) == 0 {
		panic("No PGHOST environment variable")
	}

	queueTopic = os.Getenv("QUEUE_TOPIC")
	if len(queueTopic) == 0 {
		panic("No QUEUE_TOPIC environment variable")
	}

	uploadFolderPath = os.Getenv("UPLOAD_FOLDER_PATH")
	if len(uploadFolderPath) == 0 {
		panic("No UPLOAD_FOLDER_PATH environment variable")
	}
}

func startAPIServer() {
	// Creates a gin router with default middleware:
	// logger and recovery (crash-free) middleware
	router := gin.Default()

	v1 := router.Group("/api/v1")
	{
		v1.GET("/videos", getVideoList)
		v1.GET("/videos/:id", getVideoDetail)
		v1.POST("/videos", createVideo)
		v1.POST("/video-upload", uploadVideoFile)
	}

	// By default it serves on :8080
	router.Run()
}

func getVideoList(c *gin.Context) {
	connection := GetDatabaseConnection(pgUser, pgPassword, pgHost, pgDb)
	count, videos, err := GetVideoObjects(connection)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"count":   count,
			"results": videos,
		})
	}
}

func getVideoDetail(c *gin.Context) {
	videoID, err := strconv.Atoi(c.Param("id"))

	connection := GetDatabaseConnection(pgUser, pgPassword, pgHost, pgDb)
	video, err := GetVideoObject(videoID, connection)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"data": video,
		})
	}
}

func createVideo(c *gin.Context) {
	var videoSerializer Video

	if err := c.BindJSON(&videoSerializer); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})

		return
	}

	connection := GetDatabaseConnection(pgUser, pgPassword, pgHost, pgDb)
	videoSerializer, err := CreateVideoObject(videoSerializer, connection)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"title":   videoSerializer.Title,
		"message": "Object created. Please upload the file for this Video.",
	})
}

func uploadVideoFile(c *gin.Context) {
	// Setting the maximum form data size for big file upload
	c.Request.ParseMultipartForm(64 << 25)

	videoID := c.Request.FormValue("video_id")
	if videoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "video_id is required",
		})

		return
	}

	file, header, err := c.Request.FormFile("upload")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   err.Error(),
			"message": "Please upload file with 'upload' form field key.",
		})

		return
	}

	filename := header.Filename
	outFile, err := os.Create(uploadFolderPath + filename)
	if err != nil {
		glog.Fatalln("Failed to write filesystem:", err)
	}

	defer outFile.Close()

	_, err = io.Copy(outFile, file)
	if err != nil {
		glog.Fatalln("Failed to copy video file:", err)

		c.JSON(http.StatusBadRequest, gin.H{
			"error":   err.Error(),
			"message": "File upload is having issues right now. Please try later.",
		})

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Video file uploaded. Transcoding now: %s", videoID),
	})
}
