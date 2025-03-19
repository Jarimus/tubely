package main

import (
	"fmt"
	"io"
	"net/http"

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

	// TODO: implement the upload here
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
	}

	dbVideo, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to get video meta data from database", err)
	}

	if dbVideo.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not your video", nil)
	}

	videoThumb := thumbnail{
		data:      data,
		mediaType: header.Header.Get("Content-Type"),
	}
	videoThumbnails[videoID] = videoThumb

	thumbnailUrl := fmt.Sprintf("http://localhost:8091/api/thumbnails/%s", videoIDString)

	dbVideo.ThumbnailURL = &thumbnailUrl

	err = cfg.db.UpdateVideo(dbVideo)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to update video data in database", err)
	}

	respondWithJSON(w, http.StatusOK, dbVideo)
}
