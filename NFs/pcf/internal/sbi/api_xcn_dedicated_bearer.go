package sbi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/free5gc/pcf/internal/sbi/processor"
)

func (s *Server) getXcnDedicatedBearerRoutes() []Route {
	return []Route{
		{
			Name:    "CreateXcnDedicatedBearer",
			Method:  http.MethodPost,
			Pattern: "/bearers",
			APIFunc: s.HTTPCreateXcnDedicatedBearer,
		},
		{
			Name:    "QueryXcnDedicatedBearer",
			Method:  http.MethodGet,
			Pattern: "/bearers",
			APIFunc: s.HTTPQueryXcnDedicatedBearer,
		},
		{
			Name:    "DeleteXcnDedicatedBearer",
			Method:  http.MethodDelete,
			Pattern: "/bearers/:appSessionId",
			APIFunc: s.HTTPDeleteXcnDedicatedBearer,
		},
		{
			Name:    "DeleteXcnDedicatedBearerByRequest",
			Method:  http.MethodPost,
			Pattern: "/bearers/delete",
			APIFunc: s.HTTPDeleteXcnDedicatedBearerByRequest,
		},
	}
}

func (s *Server) HTTPCreateXcnDedicatedBearer(c *gin.Context) {
	var request processor.XcnDedicatedBearerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.Processor().HandleCreateXcnDedicatedBearer(c, request)
}

func (s *Server) HTTPQueryXcnDedicatedBearer(c *gin.Context) {
	request, ok := bindXcnDedicatedBearerQuery(c)
	if !ok {
		return
	}
	s.Processor().HandleQueryXcnDedicatedBearer(c, request)
}

func (s *Server) HTTPDeleteXcnDedicatedBearer(c *gin.Context) {
	s.Processor().HandleDeleteXcnDedicatedBearerByID(c, c.Param("appSessionId"))
}

func (s *Server) HTTPDeleteXcnDedicatedBearerByRequest(c *gin.Context) {
	var request processor.XcnDedicatedBearerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.Processor().HandleDeleteXcnDedicatedBearerByRequest(c, request)
}

func bindXcnDedicatedBearerQuery(c *gin.Context) (processor.XcnDedicatedBearerRequest, bool) {
	request := processor.XcnDedicatedBearerRequest{
		Supi: c.Query("supi"),
		UeIP: firstQuery(c, "ueIp", "ueIpAddr", "ueIpv4", "ueIpv6"),
		Dnn:  c.Query("dnn"),
	}
	var ok bool
	if request.PduSessionID, ok = parseInt32Query(c, "pduSessionId"); !ok {
		return request, false
	}
	if request.NgapID, ok = parseInt64Query(c, "ngapId"); !ok {
		return request, false
	}
	if request.AmfUeNgapID, ok = parseInt64Query(c, "amfUeNgapId"); !ok {
		return request, false
	}
	if request.RanUeNgapID, ok = parseInt64Query(c, "ranUeNgapId"); !ok {
		return request, false
	}
	return request, true
}

func firstQuery(c *gin.Context, keys ...string) string {
	for _, key := range keys {
		if val := c.Query(key); val != "" {
			return val
		}
	}
	return ""
}

func parseInt32Query(c *gin.Context, key string) (int32, bool) {
	raw := c.Query(key)
	if raw == "" {
		return 0, true
	}
	parsed, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": key + " must be an integer"})
		return 0, false
	}
	return int32(parsed), true
}

func parseInt64Query(c *gin.Context, key string) (int64, bool) {
	raw := c.Query(key)
	if raw == "" {
		return 0, true
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": key + " must be an integer"})
		return 0, false
	}
	return parsed, true
}
