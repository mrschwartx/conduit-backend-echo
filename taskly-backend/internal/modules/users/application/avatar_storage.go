package application

import "context"

type AvatarStorage interface {
	Remove(ctx context.Context, avatarPath string)
}
