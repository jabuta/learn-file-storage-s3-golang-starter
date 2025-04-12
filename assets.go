package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func getAssetPath(mediaType string) string {
	extension := mediaTypeToExt(mediaType)

	b := make([]byte, 32)
	rand.Read(b)
	filename := base64.URLEncoding.EncodeToString(b)
	return fmt.Sprintf("%s%s", filename, extension)
}

func mediaTypeToExt(mediaType string) string {
	parts := strings.Split(mediaType, "/")
	if len(parts) != 2 {
		return ".bin"
	}
	return "." + parts[1]
}

func (cfg apiConfig) getAssetDiskPath(assetPath string) string {
	return filepath.Join(cfg.assetsRoot, assetPath)
}

func (cfg apiConfig) getAssetURL(assetPath string) string {
	return fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, assetPath)

}

func (cfg apiConfig) getOldDiskPath(thumbailURL string) string {
	return strings.TrimPrefix(thumbailURL, fmt.Sprintf("http://localhost:%s/assets/", cfg.port))
}

func getMediaTypeFromHeader(header *multipart.FileHeader) (string, error) {
	contentType := header.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", err
	}
	return mediaType, nil
}

// Video url is returned as cloudfront distrubition/key
func (cfg apiConfig) getVideoURL(key string) string {
	return fmt.Sprintf("%s/%s",
		cfg.s3CfDistribution,
		key)
}

func getVideoAspectRatio(filePath string) (string, error) {

	ffCommand := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var videodata bytes.Buffer

	ffCommand.Stdout = &videodata

	if err := ffCommand.Run(); err != nil {
		return "", err
	}

	type Stream struct {
		Width       int    `json:"width"`
		Height      int    `json:"height"`
		AspectRatio string `json:"display_aspect_ratio"`
		CodecType   string `json:"codec_type"`
	}

	type MediaInfo struct {
		Streams []Stream `json:"streams"`
	}
	var videoJsonData MediaInfo

	if err := json.Unmarshal(videodata.Bytes(), &videoJsonData); err != nil {
		return "", err
	}
	for _, stream := range videoJsonData.Streams {
		if stream.CodecType == "video" {
			switch stream.AspectRatio {
			case "16:9":
				return "landscape", nil
			case "9:16":
				return "portrait", nil
			default:
				return "other", nil
			}
		}
	}
	return "", errors.New("not a video file")
}

/*
func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s3Client)
	presignedRequest, err := presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expireTime))
	if err != nil {
		return "", err
	}
	return presignedRequest.URL, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	videoURL := strings.Split(*video.VideoURL, ",")
	url, err := generatePresignedURL(cfg.s3Client, videoURL[0], videoURL[1], time.Hour*2)
	if err != nil {
		return video, err
	}

	video.VideoURL = &url

	return video, nil
}
*/
