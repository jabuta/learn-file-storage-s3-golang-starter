package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading video ", videoID, "by user", userID)

	const maxMemory int64 = 10 << 30

	err = r.ParseMultipartForm(maxMemory)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Form cant be parsed", err)
		return
	}

	uploadedVideo, videoHeader, err := r.FormFile("video")

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "File cant be retreived", err)
		return
	}

	defer uploadedVideo.Close()

	mediatype, err := getMediaTypeFromHeader(videoHeader)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid media type", err)
		return
	}
	if mediatype != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "only mp4 allowed", err)
		return
	}

	tempVideo, err := os.CreateTemp("", fmt.Sprint(videoIDString, "-temp.mp4"))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "temp file failed to create", err)
		return
	}
	defer os.Remove(tempVideo.Name())
	defer tempVideo.Close()

	if _, err := io.Copy(tempVideo, uploadedVideo); err != nil {
		respondWithError(w, http.StatusInternalServerError, "temp file failed to copy", err)
		return
	}
	processedVideo, err := processVideoForFastStart(tempVideo.Name())
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid video file", err)
		return
	}
	defer os.Remove(processedVideo)
	processedVideoFile, err := os.Open(processedVideo)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "cant open processed video", err)
		return
	}
	defer processedVideoFile.Close()

	aspectRatio, err := getVideoAspectRatio(processedVideoFile.Name())
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid video", err)
	}
	assetkey := aspectRatio + "/" + getAssetPath(mediatype)

	upload := &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(assetkey),
		Body:        processedVideoFile,
		ContentType: aws.String(mediatype),
	}

	if _, err = cfg.s3Client.PutObject(context.TODO(), upload); err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed s3 upload", err)
		return
	}

	videoURL := cfg.getVideoURL(assetkey)
	video.VideoURL = &videoURL

	if err := cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to update database", err)
		return
	}
	respondWithJSON(w, http.StatusOK, video)
}

func processVideoForFastStart(filePath string) (string, error) {
	processedFilePath := filePath + ".processing"
	cmd := exec.Command("ffmpeg",
		"-i", filePath,
		"-c", "copy",
		"-movflags", "faststart",
		"-f", "mp4",
		processedFilePath)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("error processing video: %s, %v", stderr.String(), err)
	}

	fileInfo, err := os.Stat(processedFilePath)
	if err != nil {
		return "", fmt.Errorf("could not stat processed file: %v", err)
	}
	if fileInfo.Size() == 0 {
		return "", fmt.Errorf("processed file is empty")
	}

	return processedFilePath, nil
}
