package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {

	// Set max bytes to read (1 GB)
	reader := http.MaxBytesReader(w, r.Body, 1<<30)
	defer reader.Close()

	// Extract videoID from url
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	// Authenticate user
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	// Get video metadata from the database and ensure the user is the owner of the video
	dbVideo, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to get video meta data from database", err)
		return
	}

	if dbVideo.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not your video", nil)
		return
	}

	// Parse the uploaded video from the form data
	videoUploadFile, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer videoUploadFile.Close()

	// Check the media type, and accept only video/mp4
	mediatype, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error parsing media type", err)
	}
	if mediatype != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "invalid file type", nil)
		return
	}

	// Save a temporary file on disk
	tempVideoFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error creating temp file", err)
	}
	defer os.Remove("tubely-upload.mp4")
	defer tempVideoFile.Close()

	io.Copy(tempVideoFile, videoUploadFile)

	_, err = tempVideoFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error resetting temp file pointer", err)
	}

	// Necessary info to put the video into s3
	bucketName := cfg.s3Bucket
	region := cfg.s3Region

	bytes := make([]byte, 32)
	_, err = rand.Read(bytes)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error creating thumbnail url", err)
	}
	randomString := base64.RawURLEncoding.EncodeToString(bytes)
	filetype := strings.Split(mediatype, "/")[1]
	key := fmt.Sprintf("%s.%s", randomString, filetype)

	putObjectInput := s3.PutObjectInput{
		Bucket:      &bucketName,
		Key:         &key,
		Body:        tempVideoFile,
		ContentType: &mediatype,
	}

	_, err = cfg.s3Client.PutObject(r.Context(), &putObjectInput)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error putting to s3", err)
	}

	dbUrl := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", bucketName, region, key)
	dbVideo.VideoURL = &dbUrl

	err = cfg.db.UpdateVideo(dbVideo)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to update video data in database", err)
		return
	}

}
