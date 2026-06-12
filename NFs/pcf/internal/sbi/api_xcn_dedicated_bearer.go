package sbi

import (
	"net/http"

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
