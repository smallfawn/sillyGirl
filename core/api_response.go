package core

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func ApiOK(ctx *gin.Context, data interface{}) {
	apiSuccess(ctx, http.StatusOK, data)
}

func ApiCreated(ctx *gin.Context, location string, data interface{}) {
	if location = strings.TrimSpace(location); location != "" {
		ctx.Header("Location", location)
	}
	apiSuccess(ctx, http.StatusCreated, data)
}

func ApiAccepted(ctx *gin.Context, location string, data interface{}) {
	if location = strings.TrimSpace(location); location != "" {
		ctx.Header("Location", location)
	}
	apiSuccess(ctx, http.StatusAccepted, data)
}

func apiSuccess(ctx *gin.Context, httpStatus int, data interface{}) {
	apiResponse(ctx, httpStatus, true, "成功", data)
}

func ApiList(ctx *gin.Context, list interface{}, total int, extras ...map[string]interface{}) {
	data := gin.H{
		"list":  list,
		"total": total,
	}
	for _, extra := range extras {
		for key, value := range extra {
			data[key] = value
		}
	}
	ApiOK(ctx, data)
}

func ApiFail(ctx *gin.Context, message string) {
	ApiError(ctx, http.StatusBadRequest, message)
}

func ApiError(ctx *gin.Context, httpStatus int, message string) {
	ApiProblem(ctx, httpStatus, message, nil)
}

func ApiProblem(ctx *gin.Context, httpStatus int, message string, extensions map[string]interface{}) {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "请求失败"
	}
	if httpStatus == 0 {
		httpStatus = http.StatusInternalServerError
	}
	var data interface{}
	if len(extensions) != 0 {
		data = extensions
	}
	apiResponse(ctx, httpStatus, false, message, data)
}

func apiResponse(ctx *gin.Context, httpStatus int, status bool, message string, data interface{}) {
	ctx.JSON(httpStatus, gin.H{
		"status":  status,
		"message": message,
		"data":    data,
	})
}

func ApiUnauthorized(ctx *gin.Context, message string) {
	ApiError(ctx, http.StatusUnauthorized, message)
}

func ApiForbidden(ctx *gin.Context, message string) {
	ApiError(ctx, http.StatusForbidden, message)
}

func ApiNotFound(ctx *gin.Context, message string) {
	ApiError(ctx, http.StatusNotFound, message)
}

func ApiConflict(ctx *gin.Context, message string) {
	ApiError(ctx, http.StatusConflict, message)
}

func ApiUnprocessable(ctx *gin.Context, message string) {
	ApiError(ctx, http.StatusUnprocessableEntity, message)
}

func ApiValidationError(ctx *gin.Context, message string, errors interface{}) {
	ApiProblem(ctx, http.StatusUnprocessableEntity, message, map[string]interface{}{"errors": errors})
}

func ApiInternalError(ctx *gin.Context, message string) {
	ApiError(ctx, http.StatusInternalServerError, message)
}

func ApiBadGateway(ctx *gin.Context, message string) {
	ApiError(ctx, http.StatusBadGateway, message)
}
