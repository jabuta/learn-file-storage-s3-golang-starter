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

func (cfg apiConfig) getVideoURL(key string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s",
		cfg.s3Bucket,
		cfg.s3Region,
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
	fmt.Printf("work 1")
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
	return "", errors.New("Not a video file")
}
