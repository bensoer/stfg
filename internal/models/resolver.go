package models

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"stfg/internal"

	"go.uber.org/zap"
)

var RemoteModelMap = map[string]string{
	EmbeddingModel: "https://huggingface.co/Qwen/Qwen3-Embedding-0.6B-GGUF/resolve/main/Qwen3-Embedding-0.6B-Q8_0.gguf",
}

var LocalNameModelMap = map[string]string{
	EmbeddingModel: "Qwen3-Embedding-0.6B-Q8_0.gguf",
}

func ResolveModel(modelName string) (string, error) {

	targetPath := fmt.Sprintf("models/%s", LocalNameModelMap[modelName])

	// Check if models/${modelName}.gguf exists
	if !internal.FileExists(targetPath) {
		zap.S().Info("Model not found locally, downloading...", zap.String("modelName", modelName))
		if err := DownloadFile(RemoteModelMap[modelName], targetPath); err != nil {
			return "", fmt.Errorf("failed to download model %s: %w", modelName, err)
		}
	}

	// return the absolute path
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return "", err
	}
	return absPath, nil

}

func DownloadFile(url string, targetPath string) error {
	zap.S().Info("Downloading file...", zap.String("url", url), zap.String("targetPath", targetPath))
	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	var start int64

	// Check if partial file exists
	if info, err := os.Stat(targetPath); err == nil {
		start = info.Size()
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	// Resume download
	if start > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", start))
	}

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// If server ignored Range request, restart
	if start > 0 && resp.StatusCode == http.StatusOK {
		start = 0

		file, err := os.Create(targetPath)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(file, resp.Body)
		return err
	}

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	var file *os.File

	if start > 0 {
		file, err = os.OpenFile(targetPath, os.O_APPEND|os.O_WRONLY, 0644)
	} else {
		file, err = os.Create(targetPath)
	}

	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}
