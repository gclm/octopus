package handlers

import (
	"net/http"
	"time"

	"github.com/bestruirui/octopus/internal/op"
	"github.com/bestruirui/octopus/internal/server/middleware"
	"github.com/bestruirui/octopus/internal/server/resp"
	"github.com/bestruirui/octopus/internal/server/router"
	"github.com/gin-gonic/gin"
)

func init() {
	router.NewGroupRouter("/api/v1/stats").
		Use(middleware.Auth()).
		AddRoute(
			router.NewRoute("/today", http.MethodGet).
				Handle(getStatsToday),
		).
		AddRoute(
			router.NewRoute("/daily", http.MethodGet).
				Handle(getStatsDaily),
		).
		AddRoute(
			router.NewRoute("/hourly", http.MethodGet).
				Handle(getStatsHourly),
		).
		AddRoute(
			router.NewRoute("/total", http.MethodGet).
				Handle(getStatsTotal),
		).
		AddRoute(
			router.NewRoute("/apikey", http.MethodGet).
				Handle(getStatsAPIKey),
		).
		AddRoute(
			router.NewRoute("/range", http.MethodGet).
				Handle(getStatsRange),
		).
		AddRoute(
			router.NewRoute("/models", http.MethodGet).
				Handle(getStatsModels),
		)
}

// getStatsRange 按时间范围查询聚合统计
func getStatsRange(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	if startDate == "" || endDate == "" {
		resp.Error(c, http.StatusBadRequest, "start_date and end_date are required")
		return
	}

	// 验证日期格式
	if _, err := time.Parse("20060102", startDate); err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid start_date format, expected YYYYMMDD")
		return
	}
	if _, err := time.Parse("20060102", endDate); err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid end_date format, expected YYYYMMDD")
		return
	}

	result, err := op.StatsGetByRange(c.Request.Context(), startDate, endDate)
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp.Success(c, result)
}

// getStatsModels 按时间范围查询模型统计
func getStatsModels(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	if startDate == "" || endDate == "" {
		resp.Error(c, http.StatusBadRequest, "start_date and end_date are required")
		return
	}

	// 验证日期格式
	if _, err := time.Parse("20060102", startDate); err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid start_date format, expected YYYYMMDD")
		return
	}
	if _, err := time.Parse("20060102", endDate); err != nil {
		resp.Error(c, http.StatusBadRequest, "invalid end_date format, expected YYYYMMDD")
		return
	}

	result, err := op.StatsModelDailyGetAggregatedByRange(c.Request.Context(), startDate, endDate)
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp.Success(c, result)
}

func getStatsToday(c *gin.Context) {
	resp.Success(c, op.StatsTodayGet())
}

func getStatsDaily(c *gin.Context) {
	statsDaily, err := op.StatsGetDaily(c.Request.Context())
	if err != nil {
		resp.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp.Success(c, statsDaily)
}

func getStatsHourly(c *gin.Context) {
	resp.Success(c, op.StatsHourlyGet())
}

func getStatsTotal(c *gin.Context) {
	resp.Success(c, op.StatsTotalGet())
}

func getStatsAPIKey(c *gin.Context) {
	resp.Success(c, op.StatsAPIKeyList())
}
