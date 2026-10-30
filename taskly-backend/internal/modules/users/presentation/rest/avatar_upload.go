package rest

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	sharedomain "taskly/backend/internal/shared/domain"
	"taskly/backend/internal/shared/infrastructure/httpx"
)

const (
	avatarFieldName     = "avatar"
	multipartOverhead   = 1 << 20
	maxRandomSuffix     = 1_000_000_000
	unsupportedTypeText = "Only JPEG, PNG, WEBP and GIF images are allowed"
	malformedUploadText = "Malformed multipart body"
	unexpectedFieldText = "Unexpected field"
	tooManyFilesText    = "Too many files"
	bytesPerMegabyte    = 1024 * 1024
)

var extensionByMimeType = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

type AvatarUploader struct {
	dir      string
	maxBytes int64
}

func NewAvatarUploader(dir string, maxBytes int64) *AvatarUploader {
	return &AvatarUploader{dir: dir, maxBytes: maxBytes}
}

func (u *AvatarUploader) Receive(c *echo.Context) (filename string, err error) {
	request := c.Request()
	request.Body = http.MaxBytesReader(c.Response(), request.Body, u.maxBytes+multipartOverhead)

	reader, err := request.MultipartReader()
	if err != nil {
		if errors.Is(err, http.ErrNotMultipart) || errors.Is(err, http.ErrMissingBoundary) {
			return "", nil
		}
		return "", sharedomain.NewValidationError(malformedUploadText)
	}

	defer func() {
		if err != nil && filename != "" {
			_ = os.Remove(filepath.Join(u.dir, filename))
			filename = ""
		}
	}()

	if err = os.MkdirAll(u.dir, 0o755); err != nil {
		return "", err
	}

	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			return filename, nil
		}
		if nextErr != nil {
			return filename, translateMultipartError(nextErr)
		}
		if part.FileName() == "" {
			if _, err = io.Copy(io.Discard, part); err != nil {
				return filename, translateMultipartError(err)
			}
			continue
		}
		if part.FormName() != avatarFieldName {
			return filename, sharedomain.NewValidationError(unexpectedFieldText)
		}
		if filename != "" {
			return filename, sharedomain.NewValidationError(tooManyFilesText)
		}
		filename, err = u.store(part)
		if err != nil {
			return filename, err
		}
	}
}

func (u *AvatarUploader) store(part *multipart.Part) (string, error) {
	extension, ok := detectExtension(part.Header.Get("Content-Type"))
	if !ok {
		return "", sharedomain.NewValidationError(unsupportedTypeText)
	}

	name, err := newAvatarFilename(extension)
	if err != nil {
		return "", err
	}
	target, err := os.OpenFile(filepath.Join(u.dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}

	written, copyErr := io.Copy(target, io.LimitReader(part, u.maxBytes+1))
	closeErr := target.Close()
	switch {
	case copyErr != nil:
		_ = os.Remove(target.Name())
		return "", translateMultipartError(copyErr)
	case closeErr != nil:
		_ = os.Remove(target.Name())
		return "", closeErr
	case written > u.maxBytes:
		_ = os.Remove(target.Name())
		return "", sharedomain.NewValidationError(
			fmt.Sprintf("File size exceeds %dMB", u.maxBytes/bytesPerMegabyte),
		)
	}
	return name, nil
}

func translateMultipartError(err error) error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return httpx.NewPayloadTooLargeError()
	}
	return sharedomain.NewValidationError(malformedUploadText)
}

func detectExtension(contentType string) (string, bool) {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil {
		return "", false
	}
	extension, ok := extensionByMimeType[strings.ToLower(mediaType)]
	return extension, ok
}

func newAvatarFilename(extension string) (string, error) {
	suffix, err := rand.Int(rand.Reader, big.NewInt(maxRandomSuffix))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("avatar-%d-%d%s", time.Now().UnixMilli(), suffix.Int64(), extension), nil
}
