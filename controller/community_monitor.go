package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func GetCommunityMonitorConfig(c *gin.Context) {
	config, err := service.GetCommunityMonitorConfig()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, config)
}

func UpdateCommunityMonitorConfig(c *gin.Context) {
	var config service.CommunityMonitorConfig
	if err := common.UnmarshalBodyReusable(c, &config); err != nil {
		common.ApiError(c, err)
		return
	}
	publicConfig, err := service.SaveCommunityMonitorConfig(config)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, publicConfig)
}

func GetCommunityMonitorStatus(c *gin.Context) {
	status, err := service.GetCommunityMonitorStatus()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, status)
}

func GetCommunityMonitorResults(c *gin.Context) {
	results, err := service.GetCommunityMonitorResults()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, results)
}

func ScanCommunityMonitor(c *gin.Context) {
	status, err := service.ScanCommunityMonitor()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, status)
}

func DetectCommunityMonitor(c *gin.Context) {
	status, err := service.DetectCommunityMonitorCandidates()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, status)
}

func StartCommunityMonitorCollector(c *gin.Context) {
	status, err := service.StartCommunityMonitorCollector()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, status)
}

func StopCommunityMonitorCollector(c *gin.Context) {
	status, err := service.StopCommunityMonitorCollector()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, status)
}
