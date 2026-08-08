package core

import (
	"bytes"
	"encoding/base64"
	"image"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func Base642Binary(c *gin.Context) {
	random := c.Param("random")
	s, ok := temp.Get("base64_" + random).(string)
	if !ok {
		ApiNotFound(c, "临时二进制内容不存在")
		return
	}
	input := strings.TrimPrefix(s, "base64://")
	data, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		ApiInternalError(c, "临时二进制内容格式无效")
		return
	}
	// 解析图片格式
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	contentType := "application/octet-stream"
	if err != nil {
		contentType = "application/octet-stream"
	} else {
		// 根据图片格式设置响应头
		switch format {
		case "jpeg":
			contentType = "image/jpeg"
		case "png":
			contentType = "image/png"
		default:
			contentType = "application/octet-stream"
		}
	}
	c.Data(http.StatusOK, contentType, data)
}
