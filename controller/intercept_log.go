package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func GetAllInterceptLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)

	var autoDisabled *bool
	if v := c.Query("auto_disabled_channel"); v != "" {
		b := v == "true" || v == "1"
		autoDisabled = &b
	}

	upstreamStatusCode, _ := strconv.Atoi(c.Query("upstream_status_code"))
	channelId, _ := strconv.Atoi(c.Query("channel_id"))
	channelType, _ := strconv.Atoi(c.Query("channel_type"))
	userId, _ := strconv.Atoi(c.Query("user_id"))
	tokenId, _ := strconv.Atoi(c.Query("token_id"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	params := model.InterceptLogQueryParams{
		RequestId:           c.Query("request_id"),
		UserId:              userId,
		TokenId:             tokenId,
		ChannelId:           channelId,
		ChannelType:         channelType,
		ModelName:           c.Query("model_name"),
		RequestPath:         c.Query("request_path"),
		InterceptType:       c.Query("intercept_type"),
		Rule:                c.Query("rule"),
		Keyword:             c.Query("keyword"),
		Severity:            c.Query("severity"),
		AutoDisabledChannel: autoDisabled,
		UpstreamStatusCode:  upstreamStatusCode,
		StartTimestamp:      startTimestamp,
		EndTimestamp:        endTimestamp,
	}

	logs, total, err := model.GetAllInterceptLogs(params, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
}

func GetInterceptLogDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "invalid id",
		})
		return
	}
	log, err := model.GetInterceptLogById(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    log,
	})
}

func DeleteInterceptLogs(c *gin.Context) {
	targetTimestamp, _ := strconv.ParseInt(c.Query("target_timestamp"), 10, 64)
	if targetTimestamp == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "target_timestamp is required",
		})
		return
	}
	if targetTimestamp > time.Now().Unix() {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "target_timestamp cannot be in the future",
		})
		return
	}
	count, err := model.DeleteOldInterceptLogs(targetTimestamp, 100, 10000)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    count,
	})
}
