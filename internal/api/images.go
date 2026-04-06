package api

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sderosiaux/linkedin-ads-cli/internal/client"
	"github.com/sderosiaux/linkedin-ads-cli/internal/urn"
)

// UploadImageResult contains the URN assigned to the uploaded image and the
// upload URL used (mostly for debugging).
type UploadImageResult struct {
	ImageURN  string
	UploadURL string
}

// UploadImage performs the two-step LinkedIn image upload:
//  1. POST /images?action=initializeUpload to obtain an upload URL and image URN.
//  2. PUT the binary file content to the upload URL.
func UploadImage(ctx context.Context, c *client.Client, filePath, ownerURN, accountID, assetName string) (*UploadImageResult, error) {
	// Step 1: initialize upload
	initReq := map[string]any{
		"owner": ownerURN,
	}
	if accountID != "" && assetName != "" {
		initReq["mediaLibraryMetadata"] = map[string]any{
			"associatedAccount": urn.Wrap(urn.Account, accountID),
			"assetName":         assetName,
		}
	}
	initBody := map[string]any{
		"initializeUploadRequest": initReq,
	}
	var initResp struct {
		Value struct {
			UploadURL string `json:"uploadUrl"`
			Image     string `json:"image"`
		} `json:"value"`
	}
	if _, err := c.PostJSON(ctx, "/images?action=initializeUpload", initBody, &initResp); err != nil {
		return nil, fmt.Errorf("initialize upload: %w", err)
	}

	// Step 2: PUT binary
	fileBytes, err := os.ReadFile(filePath) //nolint:gosec // user-provided file path
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	contentType := detectMIME(filePath)
	if err := c.PutBinary(ctx, initResp.Value.UploadURL, fileBytes, contentType); err != nil {
		return nil, fmt.Errorf("upload binary: %w", err)
	}

	return &UploadImageResult{
		ImageURN:  initResp.Value.Image,
		UploadURL: initResp.Value.UploadURL,
	}, nil
}

// detectMIME returns the MIME content type based on the file extension.
func detectMIME(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}
