package infrastructure

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"taskly/backend/internal/modules/users/domain"
)

type LocalAvatarStorage struct {
	publicDir string
	logger    *slog.Logger
}

func NewLocalAvatarStorage(publicDir string, logger *slog.Logger) *LocalAvatarStorage {
	return &LocalAvatarStorage{publicDir: publicDir, logger: logger}
}

func (s *LocalAvatarStorage) Remove(_ context.Context, avatarPath string) {
	if !domain.IsCustomAvatar(avatarPath) {
		return
	}
	target := filepath.Join(s.publicDir, filepath.FromSlash(avatarPath))

	info, err := os.Lstat(target)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err == nil && !info.Mode().IsRegular() {
		err = errors.New("avatar path is not a regular file")
	}
	if err == nil {
		err = os.Remove(target)
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		s.logger.Warn("Failed to remove avatar file", "avatarPath", avatarPath, "message", err.Error())
	}
}
