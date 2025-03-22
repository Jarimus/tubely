package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const MaxMemory = 10 << 20
	err = r.ParseMultipartForm(MaxMemory)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't upload thumbnail", err)
		return
	}

	// "thumbnail" should match the HTML form input name
	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to read form file to memory", err)
		return
	}

	dbVideo, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to get video meta data from database", err)
		return
	}

	if dbVideo.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not your video", nil)
		return
	}

	// Store the thumbnail to /assets/<randomstring>.<file_extension>
	mediatype, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error parsing media type", err)
	}
	if (mediatype != "image/jpeg") && (mediatype != "image/png") {
		respondWithError(w, http.StatusBadRequest, "invalid file type", nil)
		return
	}

	bytes := make([]byte, 32)
	_, err = rand.Read(bytes)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error creating thumbnail url", err)
	}
	randomString := base64.RawURLEncoding.EncodeToString(bytes)
	filetype := strings.Split(mediatype, "/")[1]
	filename := fmt.Sprintf("%s.%s", randomString, filetype)
	filepath := filepath.Join(cfg.assetsRoot, filename)
	thumbnailFile, err := os.Create(filepath)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error creating file", err)
		return
	}
	_, err = thumbnailFile.Write(data)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error writing to file system", err)
		return
	}

	thumbnailUrl := fmt.Sprintf("http://localhost:8091/assets/%s.%s", randomString, filetype)
	dbVideo.ThumbnailURL = &thumbnailUrl

	err = cfg.db.UpdateVideo(dbVideo)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to update video data in database", err)
		return
	}

	respondWithJSON(w, http.StatusOK, dbVideo)
}
