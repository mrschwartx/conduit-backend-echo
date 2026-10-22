package comments

import (
	"conduit/backend/internal/platform/httpx"
	"conduit/backend/internal/profiles"
)

// NewCommentInput is the "comment" object of POST /articles/{slug}/comments.
type NewCommentInput struct {
	Body string `json:"body"`
}

// CreateRequest is the request envelope.
type CreateRequest struct {
	Comment *NewCommentInput `json:"comment"`
}

// CommentDTO matches the contract "Comment" schema (id is an integer).
type CommentDTO struct {
	ID        int64               `json:"id"`
	CreatedAt string              `json:"createdAt"`
	UpdatedAt string              `json:"updatedAt"`
	Body      string              `json:"body"`
	Author    profiles.ProfileDTO `json:"author"`
}

// CommentResponse is the {"comment": ...} envelope.
type CommentResponse struct {
	Comment CommentDTO `json:"comment"`
}

// CommentsResponse is the {"comments": [...]} envelope.
type CommentsResponse struct {
	Comments []CommentDTO `json:"comments"`
}

func toDTO(c *Comment) CommentDTO {
	return CommentDTO{
		ID:        c.ID,
		CreatedAt: httpx.FormatTime(c.CreatedAt),
		UpdatedAt: httpx.FormatTime(c.UpdatedAt),
		Body:      c.Body,
		Author:    profiles.ToDTO(c.Author),
	}
}
