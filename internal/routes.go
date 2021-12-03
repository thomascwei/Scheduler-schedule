package internal

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func ReturnError(c *gin.Context, statusCode int, info string) {
	c.JSON(statusCode, gin.H{
		"result": "fail",
		"error":  info,
	})
}

func UpdateScheduleRoute(c *gin.Context) {
	ID := RequestId{}
	err := c.Bind(&ID)
	if err != nil {
		ReturnError(c, http.StatusBadRequest, err.Error())
		return
	}
	err = UpdateCacheSchedule(int(ID.Id))
	if err != nil {
		ReturnError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, gin.H{
		"result": "ok",
		"data":   "",
	})
	return
}

func UpdateAllCommandsRoute(c *gin.Context) {
	err := PrepareAllCommandsStoreInCache(CRUDconfig)
	if err != nil {
		ReturnError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, gin.H{
		"result": "ok",
		"data":   "",
	})
	return
}
