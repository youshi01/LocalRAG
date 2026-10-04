package handler

import (
	"errors"
	"github.com/gin-gonic/gin"
	"localrag/internal/service"
	"net/http"
)

func (h *AppHandler) DeleteAllConversations(c *gin.Context) {
	var request struct {
		Confirm bool `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || !request.Confirm {
		writeError(c, http.StatusBadRequest, "清空全部会话需要明确确认（confirm=true）")
		return
	}
	count, err := h.appService.DeleteAllConversations()
	if err != nil {
		if errors.Is(err, service.ErrConversationGenerationActive) {
			writeError(c, http.StatusConflict, err.Error())
			return
		}
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "all conversations cleared", "deletedCount": count})
}
