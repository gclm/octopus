package middleware

import (
	"net/http"
	"strings"

	"github.com/gclm/octopus/internal/server/resp"
	"github.com/gin-gonic/gin"
)

// 分级请求体大小限制
const (
	MaxRequestBodySizeChat      = 10 * 1024 * 1024 // 10MB - 聊天请求（含多模态）
	MaxRequestBodySizeImage     = 10 * 1024 * 1024 // 10MB - 图片生成请求
	MaxRequestBodySizeEmbedding = 500 * 1024       // 500KB - Embedding 请求
	MaxRequestBodySizeDefault   = 2 * 1024 * 1024  // 2MB - 默认限制
)

func RequireJSON() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet ||
			c.Request.Method == http.MethodDelete ||
			c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		contentType := c.GetHeader("Content-Type")
		if !strings.Contains(contentType, "application/json") {
			resp.Error(c, http.StatusUnsupportedMediaType, resp.ErrInvalidJSON)
			c.Abort()
			return
		}

		// 根据请求路径设置不同的大小限制
		maxSize := getMaxRequestBodySize(c.Request.URL.Path)
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)

		c.Next()
	}
}

// getMaxRequestBodySize 根据 API 路径返回对应的请求体大小限制
func getMaxRequestBodySize(path string) int64 {
	// Embedding 请求
	if strings.Contains(path, "/embeddings") {
		return MaxRequestBodySizeEmbedding
	}
	// 图片生成请求
	if strings.Contains(path, "/images") || strings.Contains(path, "/image") {
		return MaxRequestBodySizeImage
	}
	// 聊天请求（含多模态）
	if strings.Contains(path, "/chat") || strings.Contains(path, "/completions") ||
		strings.Contains(path, "/responses") || strings.Contains(path, "/messages") {
		return MaxRequestBodySizeChat
	}
	// 默认限制
	return MaxRequestBodySizeDefault
}
