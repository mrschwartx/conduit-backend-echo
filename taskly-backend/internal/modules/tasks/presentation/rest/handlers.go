package rest

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"taskly/backend/internal/modules/tasks/application"
	"taskly/backend/internal/shared/infrastructure/httpx"
)

type MessageResponse struct {
	Message string `json:"message"`
}

type Handler struct {
	create *application.CreateTask
	get    *application.GetTask
	list   *application.ListTasksByUser
	update *application.UpdateTask
	delete *application.DeleteTask
}

func NewHandler(
	create *application.CreateTask,
	get *application.GetTask,
	list *application.ListTasksByUser,
	update *application.UpdateTask,
	deleteTask *application.DeleteTask,
) *Handler {
	return &Handler{create: create, get: get, list: list, update: update, delete: deleteTask}
}

func (h *Handler) ListByUser(c *echo.Context) error {
	query := c.QueryParams()
	result, err := h.list.Execute(c.Request().Context(), application.ListTasksInput{
		RequesterID: httpx.AuthUserID(c),
		OwnerID:     c.Param("id"),
		Status:      query["status"],
		OrderBy:     query["orderBy"],
		Page:        query["page"],
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) Get(c *echo.Context) error {
	task, err := h.get.Execute(c.Request().Context(), h.access(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, task)
}

func (h *Handler) Create(c *echo.Context) error {
	var input application.TaskInput
	if err := httpx.DecodeJSON(c, &input); err != nil {
		return err
	}
	task, err := h.create.Execute(c.Request().Context(), httpx.AuthUserID(c), input)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, task)
}

func (h *Handler) Update(c *echo.Context) error {
	var changes application.TaskInput
	if err := httpx.DecodeJSON(c, &changes); err != nil {
		return err
	}
	task, err := h.update.Execute(c.Request().Context(), application.UpdateTaskInput{
		Access:  h.access(c),
		Changes: changes,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, task)
}

func (h *Handler) Delete(c *echo.Context) error {
	if err := h.delete.Execute(c.Request().Context(), h.access(c)); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, MessageResponse{Message: "Task has been deleted"})
}

func (h *Handler) access(c *echo.Context) application.TaskAccess {
	return application.TaskAccess{RequesterID: httpx.AuthUserID(c), TaskID: c.Param("id")}
}
