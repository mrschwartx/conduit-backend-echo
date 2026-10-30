package domain

import "regexp"

const (
	AvatarURLPrefix   = "/images/users/"
	DefaultAvatarPath = AvatarURLPrefix + "default-avatar.jpg"
)

var customAvatarPattern = regexp.MustCompile(`^/images/users/avatar-[A-Za-z0-9_.-]+$`)

func IsCustomAvatar(avatarPath string) bool {
	return customAvatarPattern.MatchString(avatarPath)
}

func ToAvatarPath(filename string) string {
	return AvatarURLPrefix + filename
}
