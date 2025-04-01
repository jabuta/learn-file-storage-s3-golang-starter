package main

import (
	"fmt"
	"io"
	"net/http"
	"os"

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

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Can't fetch video data", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorized to update this video", nil)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory int64 = 10 << 20

	err = r.ParseMultipartForm(maxMemory)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Form cant be parsed", err)
		return
	}

	uploadedThumbnail, thumbnailHeader, err := r.FormFile("thumbnail")

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "File cant be retreived", err)
		return
	}
	defer uploadedThumbnail.Close()

	mediatype, err := getMediaTypeFromHeader(thumbnailHeader)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid media type", err)
		return
	}
	if mediatype != "image/png" && mediatype != "image/jpeg" {
		respondWithError(w, http.StatusBadRequest, "invalid Only jpeg or png supported", nil)
		return
	}

	assetPath := getAssetPath(mediatype)

	filepath := cfg.getAssetDiskPath(assetPath)

	fmt.Println(filepath)

	file, err := os.Create(filepath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "File can't be created", err)
		return
	}

	_, err = io.Copy(file, uploadedThumbnail)
	if err != nil {
		os.Remove(filepath)
		respondWithError(w, http.StatusInternalServerError, "File can't be saved", err)
		return
	}

	if *video.ThumbnailURL != "" {
		os.Remove(cfg.getAssetDiskPath(cfg.getOldDiskPath(*video.ThumbnailURL)))
	}

	assetURL := cfg.getAssetURL(assetPath)

	video.ThumbnailURL = &assetURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Can't save thumbnail", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
