package response

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type APIResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     *APIError   `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func JSON(c *gin.Context, httpStatus int, data interface{}) {
	c.JSON(httpStatus, APIResponse{
		Success:   httpStatus >= 200 && httpStatus < 300,
		Data:      data,
		Timestamp: time.Now().UTC(),
	})
}

func Error(c *gin.Context, httpStatus int, code string, message string, details ...string) {
	var detail string
	if len(details) > 0 {
		detail = details[0]
	}
	c.JSON(httpStatus, APIResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
			Details: detail,
		},
		Timestamp: time.Now().UTC(),
	})
}

func AbortWithError(c *gin.Context, httpStatus int, code string, message string, details ...string) {
	var detail string
	if len(details) > 0 {
		detail = details[0]
	}
	c.AbortWithStatusJSON(httpStatus, APIResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
			Details: detail,
		},
		Timestamp: time.Now().UTC(),
	})
}

func StatusText(code int) string {
	return http.StatusText(code)
}
