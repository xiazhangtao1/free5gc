package processor

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/free5gc/openapi/models"
	pcf_context "github.com/free5gc/pcf/internal/context"
	"github.com/free5gc/pcf/internal/logger"
	"github.com/free5gc/pcf/internal/util"
	"github.com/free5gc/util/metrics/sbi"
)

const xcnDedicatedBearerAfAppID = "xcn-dedicated-bearer"

type XcnDedicatedBearerRequest struct {
	AppSessionID     string         `json:"appSessionId,omitempty"`
	Supi             string         `json:"supi,omitempty"`
	PduSessionID     int32          `json:"pduSessionId,omitempty"`
	UeIP             string         `json:"ueIp,omitempty"`
	NgapID           int64          `json:"ngapId,omitempty"`
	AmfUeNgapID      int64          `json:"amfUeNgapId,omitempty"`
	RanUeNgapID      int64          `json:"ranUeNgapId,omitempty"`
	MediaType        string         `json:"mediaType,omitempty"`
	FlowDescriptions []string       `json:"flowDescriptions,omitempty"`
	Dnn              string         `json:"dnn,omitempty"`
	Snssai           *models.Snssai `json:"snssai,omitempty"`
	NotificationURI  string         `json:"notificationUri,omitempty"`
	Qos              *XcnBearerQos  `json:"qos,omitempty"`
	MarBwDl          string         `json:"marBwDl,omitempty"`
	MarBwUl          string         `json:"marBwUl,omitempty"`
	MirBwDl          string         `json:"mirBwDl,omitempty"`
	MirBwUl          string         `json:"mirBwUl,omitempty"`
	RrBw             string         `json:"rrBw,omitempty"`
	RsBw             string         `json:"rsBw,omitempty"`
}

type XcnBearerQos struct {
	Var5qi  int32         `json:"5qi,omitempty"`
	Index   int32         `json:"index,omitempty"`
	Arp     *XcnBearerArp `json:"arp,omitempty"`
	MaxbrDl string        `json:"maxbrDl,omitempty"`
	MaxbrUl string        `json:"maxbrUl,omitempty"`
	MbrDl   string        `json:"mbrDl,omitempty"`
	MbrUl   string        `json:"mbrUl,omitempty"`
	GbrDl   string        `json:"gbrDl,omitempty"`
	GbrUl   string        `json:"gbrUl,omitempty"`
}

type XcnBearerArp struct {
	PriorityLevel           int32  `json:"priorityLevel,omitempty"`
	PreemptionCapability    string `json:"preemptionCapability,omitempty"`
	PreemptionVulnerability string `json:"preemptionVulnerability,omitempty"`
}

type xcnDedicatedBearerResponse struct {
	AppSessionID string            `json:"appSessionId"`
	Location     string            `json:"location"`
	PccRuleIDs   map[string]string `json:"pccRuleIds,omitempty"`
}

type xcnDedicatedBearerQueryResponse struct {
	Supi         string                   `json:"supi"`
	PduSessionID int32                    `json:"pduSessionId"`
	UeIP         string                   `json:"ueIp,omitempty"`
	AmfUeNgapID  int64                    `json:"amfUeNgapId,omitempty"`
	RanUeNgapID  int64                    `json:"ranUeNgapId,omitempty"`
	Bearers      []xcnDedicatedBearerInfo `json:"bearers"`
	BearerCount  int                      `json:"bearerCount"`
}

type xcnDedicatedBearerInfo struct {
	AppSessionID string           `json:"appSessionId"`
	AfAppID      string           `json:"afAppId,omitempty"`
	PccRules     []xcnPccRuleInfo `json:"pccRules"`
	PccRuleCount int              `json:"pccRuleCount"`
}

type xcnPccRuleInfo struct {
	PccRuleID  string                   `json:"pccRuleId"`
	Precedence int32                    `json:"precedence"`
	FlowStatus models.FlowStatus        `json:"flowStatus"`
	Qos        *models.QosData          `json:"qos,omitempty"`
	Flows      []models.FlowInformation `json:"flows,omitempty"`
}

func (p *Processor) HandleCreateXcnDedicatedBearer(c *gin.Context, request XcnDedicatedBearerRequest) {
	smPolicy, err := p.resolveSmPolicy(request)
	if err != nil {
		xcnWriteError(c, http.StatusNotFound, err.Error())
		return
	}
	p.createXcnDedicatedBearer(c, smPolicy, request)
}

func (p *Processor) HandleCreateXcnDedicatedBearerBySupi(c *gin.Context, request XcnDedicatedBearerRequest) {
	if request.Supi == "" || request.PduSessionID == 0 {
		xcnWriteError(c, http.StatusBadRequest, "ueIp or supi and pduSessionId are required")
		return
	}
	smPolicy, ok := p.findSmPolicyBySupiAndPduSessionID(request.Supi, request.PduSessionID)
	if !ok {
		xcnWriteError(c, http.StatusNotFound, "sm policy not found")
		return
	}
	p.createXcnDedicatedBearer(c, smPolicy, request)
}

func (p *Processor) HandleCreateXcnDedicatedBearerByUeIP(c *gin.Context, request XcnDedicatedBearerRequest) {
	if request.UeIP == "" {
		xcnWriteError(c, http.StatusBadRequest, "ueIp is required")
		return
	}
	smPolicy, err := p.findSmPolicyByUeIP(request)
	if err != nil {
		xcnWriteError(c, http.StatusNotFound, err.Error())
		return
	}
	p.createXcnDedicatedBearer(c, smPolicy, request)
}

func (p *Processor) HandleQueryXcnDedicatedBearer(c *gin.Context, request XcnDedicatedBearerRequest) {
	smPolicy, err := p.resolveSmPolicy(request)
	if err != nil {
		xcnWriteError(c, http.StatusNotFound, err.Error())
		return
	}
	c.JSON(http.StatusOK, p.buildXcnDedicatedBearerQueryResponse(smPolicy))
}

func (p *Processor) HandleDeleteXcnDedicatedBearerByID(c *gin.Context, appSessionID string) {
	if appSessionID == "" {
		xcnWriteError(c, http.StatusBadRequest, "appSessionId is required")
		return
	}
	if !p.isXcnAppSession(appSessionID) {
		xcnWriteError(c, http.StatusNotFound, "xcn app session not found")
		return
	}
	p.HandleDeleteAppSessionContext(c, appSessionID, nil)
}

func (p *Processor) HandleDeleteXcnDedicatedBearerByRequest(c *gin.Context, request XcnDedicatedBearerRequest) {
	if request.AppSessionID != "" {
		p.HandleDeleteXcnDedicatedBearerByID(c, request.AppSessionID)
		return
	}
	appSessionID, err := p.findXcnAppSessionByRequest(request)
	if err != nil {
		xcnWriteError(c, http.StatusNotFound, err.Error())
		return
	}
	p.HandleDeleteAppSessionContext(c, appSessionID, nil)
}

func (p *Processor) createXcnDedicatedBearer(
	c *gin.Context,
	smPolicy *pcf_context.UeSmPolicyData,
	request XcnDedicatedBearerRequest,
) {
	if len(request.FlowDescriptions) == 0 {
		xcnWriteError(c, http.StatusBadRequest, "flowDescriptions is required")
		return
	}
	mediaType, var5qi, err := request.mediaTypeAnd5qi()
	if err != nil {
		xcnWriteError(c, http.StatusBadRequest, err.Error())
		return
	}
	medComp := request.toMediaComponent(mediaType)
	for _, medSubComp := range medComp.MedSubComps {
		if _, err = getFlowInfos(&medSubComp); err != nil {
			xcnWriteError(c, http.StatusBadRequest, err.Error())
			return
		}
	}

	relatedPccRuleIDs := make(map[string]string)
	var pccRule *models.PccRule
	for _, medSubComp := range medComp.MedSubComps {
		pccRule, err = p.createXcnPccRule(smPolicy, &medComp, &medSubComp, var5qi, request)
		if err != nil {
			xcnWriteError(c, http.StatusForbidden, err.Error())
			return
		}
		relatedPccRuleIDs[fmt.Sprintf("%d-%d", medComp.MedCompN, medSubComp.FNum)] = pccRule.PccRuleId
	}

	ue := smPolicy.PcfUe
	appSessionID := ue.AllocUeAppSessionId(p.Context())
	appSessCtx := request.toAppSessionContext(smPolicy, medComp)
	appSessCtx.AscRespData = &models.AppSessionContextRespData{
		SuppFeat: p.Context().PcfSuppFeats[models.ServiceName_NPCF_POLICYAUTHORIZATION].String(),
	}
	smPolicy.AppSessions[appSessionID] = true
	appSession := pcf_context.AppSessionData{
		AppSessionId:         appSessionID,
		AppSessionContext:    appSessCtx,
		SmPolicyData:         smPolicy,
		RelatedPccRuleIds:    relatedPccRuleIDs,
		PccRuleIdMapToCompId: reverseStringMap(relatedPccRuleIDs),
	}
	p.Context().AppSessionPool.Store(appSessionID, &appSession)

	smPolicyID := fmt.Sprintf("%s-%d", smPolicy.PcfUe.Supi, smPolicy.PolicyContext.PduSessionId)
	notification := models.SmPolicyNotification{
		ResourceUri:      util.GetResourceUri(models.ServiceName_NPCF_SMPOLICYCONTROL, smPolicyID),
		SmPolicyDecision: smPolicy.PolicyDecision,
	}
	go p.SendSMPolicyUpdateNotification(smPolicy.PolicyContext.NotificationUri, &notification)
	logger.PolicyAuthLog.Infof("XCN dedicated bearer App Session Id[%s] Create", appSessionID)

	location := util.GetResourceUri(models.ServiceName_NPCF_POLICYAUTHORIZATION, appSessionID)
	c.Header("Location", location)
	c.JSON(http.StatusCreated, xcnDedicatedBearerResponse{
		AppSessionID: appSessionID,
		Location:     location,
		PccRuleIDs:   relatedPccRuleIDs,
	})
}

func (p *Processor) createXcnPccRule(
	smPolicy *pcf_context.UeSmPolicyData,
	medComp *models.MediaComponent,
	medSubComp *models.MediaSubComponent,
	var5qi int32,
	request XcnDedicatedBearerRequest,
) (*models.PccRule, error) {
	flowInfos, err := getFlowInfos(medSubComp)
	if err != nil {
		return nil, err
	}

	id := nextAvailableXcnRuleID(smPolicy)
	precedence := getAvailablePrecedence(smPolicy.PolicyDecision.PccRules)
	pccRule := util.CreatePccRule(id, precedence, flowInfos, xcnDedicatedBearerAfAppID)
	qosData := util.CreateQosData(id, var5qi, 8)
	if var5qi <= 4 {
		var ul, dl bool
		var problemDetail *models.ProblemDetails
		qosData, ul, dl = updateQosInMedSubComp(&qosData, medComp, medSubComp)
		if problemDetail = modifyRemainBitRate(smPolicy, &qosData, ul, dl); problemDetail != nil {
			return nil, errors.New(problemDetail.Detail)
		}
	}

	for i := range flowInfos {
		flowInfos[i].PackFiltId = nextAvailableXcnPackFiltID(smPolicy)
		smPolicy.PackFiltMapToPccRuleId[flowInfos[i].PackFiltId] = pccRule.PccRuleId
	}
	pccRule.FlowInfos = flowInfos
	tcData := util.CreateTcData(id, "", medSubComp.FStatus)
	util.SetPccRuleRelatedData(smPolicy.PolicyDecision, pccRule, tcData, &qosData, nil, nil)
	smPolicy.PccRuleIdGenerator = id + 1

	for _, qosID := range pccRule.RefQosData {
		qosData := smPolicy.PolicyDecision.QosDecs[qosID]
		if qosData == nil {
			continue
		}
		if request.Qos != nil && request.Qos.Arp != nil {
			if qosData.Arp == nil {
				qosData.Arp = &models.Arp{}
			}
			if request.Qos.Arp.PriorityLevel != 0 {
				qosData.Arp.PriorityLevel = request.Qos.Arp.PriorityLevel
			}
			switch request.Qos.Arp.PreemptionCapability {
			case "NOT_PREEMPT":
				qosData.Arp.PreemptCap = models.PreemptionCapability_NOT_PREEMPT
			case "MAY_PREEMPT":
				qosData.Arp.PreemptCap = models.PreemptionCapability_MAY_PREEMPT
			}
			switch request.Qos.Arp.PreemptionVulnerability {
			case "NOT_PREEMPTABLE":
				qosData.Arp.PreemptVuln = models.PreemptionVulnerability_NOT_PREEMPTABLE
			case "PREEMPTABLE":
				qosData.Arp.PreemptVuln = models.PreemptionVulnerability_PREEMPTABLE
			}
		}
		smPolicy.PolicyDecision.QosDecs[qosID] = qosData
	}
	return pccRule, nil
}

func nextAvailableXcnRuleID(smPolicy *pcf_context.UeSmPolicyData) int32 {
	id := smPolicy.PccRuleIdGenerator
	for {
		pccRuleID := util.GetPccRuleId(id)
		qosID := util.GetQosId(id)
		tcID := util.GetTcId(id)
		_, pccExists := smPolicy.PolicyDecision.PccRules[pccRuleID]
		_, qosExists := smPolicy.PolicyDecision.QosDecs[qosID]
		_, tcExists := smPolicy.PolicyDecision.TraffContDecs[tcID]
		if !pccExists && !qosExists && !tcExists {
			return id
		}
		id++
	}
}

func nextAvailableXcnPackFiltID(smPolicy *pcf_context.UeSmPolicyData) string {
	for {
		packFiltID := util.GetPackFiltId(smPolicy.PackFiltIdGenerator)
		if _, exists := smPolicy.PackFiltMapToPccRuleId[packFiltID]; !exists {
			smPolicy.PackFiltIdGenerator++
			return packFiltID
		}
		smPolicy.PackFiltIdGenerator++
	}
}

func (p *Processor) findSmPolicyBySupiAndPduSessionID(supi string, pduSessionID int32) (*pcf_context.UeSmPolicyData, bool) {
	val, ok := p.Context().UePool.Load(supi)
	if !ok {
		return nil, false
	}
	ue := val.(*pcf_context.UeContext)
	smPolicyID := fmt.Sprintf("%s-%d", supi, pduSessionID)
	smPolicy := ue.SmPolicyData[smPolicyID]
	return smPolicy, smPolicy != nil
}

func (p *Processor) findSmPolicyByUeIP(request XcnDedicatedBearerRequest) (*pcf_context.UeSmPolicyData, error) {
	ascReqData := &models.AppSessionContextReqData{
		Supi:      request.Supi,
		Dnn:       request.Dnn,
		SliceInfo: request.Snssai,
	}
	if parsed := net.ParseIP(request.UeIP); parsed == nil {
		return nil, fmt.Errorf("invalid ueIp")
	} else if parsed.To4() != nil {
		ascReqData.UeIpv4 = request.UeIP
	} else {
		ascReqData.UeIpv6 = request.UeIP
	}
	smPolicy, err := p.Context().SessionBinding(ascReqData)
	if err != nil {
		return nil, err
	}
	return smPolicy, nil
}

func (p *Processor) resolveSmPolicy(request XcnDedicatedBearerRequest) (*pcf_context.UeSmPolicyData, error) {
	if request.UeIP != "" {
		return p.findSmPolicyByUeIP(request)
	}
	if request.AmfUeNgapID != 0 || request.RanUeNgapID != 0 || request.NgapID != 0 {
		return p.findSmPolicyByNgapID(request)
	}
	if request.Supi != "" && request.PduSessionID != 0 {
		if smPolicy, ok := p.findSmPolicyBySupiAndPduSessionID(request.Supi, request.PduSessionID); ok {
			return smPolicy, nil
		}
	}
	return nil, fmt.Errorf("sm policy not found")
}

func (p *Processor) findSmPolicyByNgapID(request XcnDedicatedBearerRequest) (*pcf_context.UeSmPolicyData, error) {
	ueContexts, err := p.Consumer().GetRegisteredUEContextsFromOAM()
	if err != nil {
		return nil, err
	}
	for _, ueContext := range ueContexts {
		if !xcnNgapSelectorMatches(request, ueContext.AmfUeNgapId, ueContext.RanUeNgapId) {
			continue
		}
		for _, pduSession := range ueContext.PduSessions {
			pduSessionID, err := strconv.ParseInt(pduSession.PduSessionId, 10, 32)
			if err != nil || pduSessionID == 0 {
				continue
			}
			if request.PduSessionID != 0 && request.PduSessionID != int32(pduSessionID) {
				continue
			}
			smPolicy, ok := p.findSmPolicyBySupiAndPduSessionID(ueContext.Supi, int32(pduSessionID))
			if ok {
				return smPolicy, nil
			}
		}
	}
	return nil, fmt.Errorf("sm policy not found")
}

func xcnNgapSelectorMatches(request XcnDedicatedBearerRequest, amfUeNgapID, ranUeNgapID int64) bool {
	if request.AmfUeNgapID != 0 && request.AmfUeNgapID == amfUeNgapID {
		return true
	}
	if request.NgapID != 0 && request.NgapID == amfUeNgapID {
		return true
	}
	if request.RanUeNgapID != 0 && request.RanUeNgapID == ranUeNgapID {
		return true
	}
	if request.NgapID != 0 && request.NgapID == ranUeNgapID {
		return true
	}
	return false
}

func (p *Processor) isXcnAppSession(appSessionID string) bool {
	val, ok := p.Context().AppSessionPool.Load(appSessionID)
	if !ok {
		return false
	}
	appSession := val.(*pcf_context.AppSessionData)
	return xcnAppSessionMatches(appSession)
}

func (p *Processor) findXcnAppSessionByRequest(request XcnDedicatedBearerRequest) (string, error) {
	smPolicy, err := p.resolveSmPolicy(request)
	if err != nil {
		return "", err
	}
	if len(request.FlowDescriptions) == 0 {
		return "", fmt.Errorf("flowDescriptions is required")
	}

	expected := sortedStrings(request.FlowDescriptions)
	for appSessionID := range smPolicy.AppSessions {
		val, ok := p.Context().AppSessionPool.Load(appSessionID)
		if !ok {
			continue
		}
		appSession := val.(*pcf_context.AppSessionData)
		if xcnAppSessionMatches(appSession) &&
			equalStringSlices(expected, sortedStrings(xcnAppSessionFlowDescriptions(appSession))) {
			return appSessionID, nil
		}
	}
	return "", fmt.Errorf("xcn app session not found")
}

func (p *Processor) buildXcnDedicatedBearerQueryResponse(
	smPolicy *pcf_context.UeSmPolicyData,
) xcnDedicatedBearerQueryResponse {
	policyContext := smPolicy.PolicyContext
	response := xcnDedicatedBearerQueryResponse{
		Supi:         smPolicy.PcfUe.Supi,
		PduSessionID: policyContext.PduSessionId,
		Bearers:      []xcnDedicatedBearerInfo{},
	}
	if policyContext.Ipv4Address != "" {
		response.UeIP = policyContext.Ipv4Address
	} else {
		response.UeIP = policyContext.Ipv6AddressPrefix
	}
	p.fillNgapIDsFromOAM(&response)

	for appSessionID := range smPolicy.AppSessions {
		val, ok := p.Context().AppSessionPool.Load(appSessionID)
		if !ok {
			continue
		}
		appSession := val.(*pcf_context.AppSessionData)
		if !xcnAppSessionMatches(appSession) {
			continue
		}
		info := xcnDedicatedBearerInfo{
			AppSessionID: appSession.AppSessionId,
			AfAppID:      appSession.AppSessionContext.AscReqData.AfAppId,
		}
		for _, pccRuleID := range appSession.RelatedPccRuleIds {
			pccRule := smPolicy.PolicyDecision.PccRules[pccRuleID]
			if pccRule == nil {
				continue
			}
			pccInfo := xcnPccRuleInfo{
				PccRuleID:  pccRule.PccRuleId,
				Precedence: pccRule.Precedence,
				Flows:      pccRule.FlowInfos,
			}
			if len(pccRule.RefQosData) > 0 {
				pccInfo.Qos = smPolicy.PolicyDecision.QosDecs[pccRule.RefQosData[0]]
			}
			if len(pccRule.RefTcData) > 0 {
				if tcData := smPolicy.PolicyDecision.TraffContDecs[pccRule.RefTcData[0]]; tcData != nil {
					pccInfo.FlowStatus = tcData.FlowStatus
				}
			}
			info.PccRules = append(info.PccRules, pccInfo)
		}
		info.PccRuleCount = len(info.PccRules)
		response.Bearers = append(response.Bearers, info)
	}
	response.BearerCount = len(response.Bearers)
	return response
}

func (p *Processor) fillNgapIDsFromOAM(response *xcnDedicatedBearerQueryResponse) {
	ueContexts, err := p.Consumer().GetRegisteredUEContextsFromOAM()
	if err != nil {
		return
	}
	for _, ueContext := range ueContexts {
		if ueContext.Supi != response.Supi {
			continue
		}
		for _, pduSession := range ueContext.PduSessions {
			pduSessionID, err := strconv.ParseInt(pduSession.PduSessionId, 10, 32)
			if err != nil || int32(pduSessionID) != response.PduSessionID {
				continue
			}
			response.AmfUeNgapID = pduSession.AmfUeNgapId
			response.RanUeNgapID = pduSession.RanUeNgapId
			return
		}
	}
}

func (r XcnDedicatedBearerRequest) mediaTypeAnd5qi() (models.MediaType, int32, error) {
	var mediaType models.MediaType
	switch strings.ToLower(r.MediaType) {
	case "", "data":
		mediaType = models.MediaType_DATA
	case "audio":
		mediaType = models.MediaType_AUDIO
	case "video":
		mediaType = models.MediaType_VIDEO
	case "application":
		mediaType = models.MediaType_APPLICATION
	case "control":
		mediaType = models.MediaType_CONTROL
	case "text":
		mediaType = models.MediaType_TEXT
	case "message":
		mediaType = models.MediaType_MESSAGE
	case "other":
		mediaType = models.MediaType_OTHER
	default:
		return "", 0, fmt.Errorf("invalid mediaType")
	}

	var5qi := int32(0)
	if r.Qos != nil {
		var5qi = r.Qos.Var5qi
		if var5qi == 0 {
			var5qi = r.Qos.Index
		}
	}
	if var5qi == 0 {
		var5qi = util.MediaTypeTo5qiMap[mediaType]
	}
	if var5qi <= 0 || var5qi > 255 {
		return "", 0, fmt.Errorf("invalid 5qi")
	}
	return mediaType, var5qi, nil
}

func (r XcnDedicatedBearerRequest) toMediaComponent(mediaType models.MediaType) models.MediaComponent {
	medComp := models.MediaComponent{
		MedCompN: 1,
		FStatus:  models.FlowStatus_ENABLED,
		MedType:  mediaType,
		MarBwDl:  r.MarBwDl,
		MarBwUl:  r.MarBwUl,
		MirBwDl:  r.MirBwDl,
		MirBwUl:  r.MirBwUl,
	}
	if r.Qos != nil {
		if r.Qos.MaxbrDl != "" {
			medComp.MarBwDl = r.Qos.MaxbrDl
		} else if r.Qos.MbrDl != "" {
			medComp.MarBwDl = r.Qos.MbrDl
		}
		if r.Qos.MaxbrUl != "" {
			medComp.MarBwUl = r.Qos.MaxbrUl
		} else if r.Qos.MbrUl != "" {
			medComp.MarBwUl = r.Qos.MbrUl
		}
		if r.Qos.GbrDl != "" {
			medComp.MirBwDl = r.Qos.GbrDl
		}
		if r.Qos.GbrUl != "" {
			medComp.MirBwUl = r.Qos.GbrUl
		}
	}
	medComp.MedSubComps = map[string]models.MediaSubComponent{
		"1": {
			FNum:    1,
			FDescs:  r.FlowDescriptions,
			FStatus: models.FlowStatus_ENABLED,
		},
	}
	return medComp
}

func (r XcnDedicatedBearerRequest) toAppSessionContext(
	smPolicy *pcf_context.UeSmPolicyData,
	medComp models.MediaComponent,
) *models.AppSessionContext {
	ascReqData := &models.AppSessionContextReqData{
		Supi:          smPolicy.PcfUe.Supi,
		AfAppId:       xcnDedicatedBearerAfAppID,
		SuppFeat:      "0",
		NotifUri:      r.NotificationURI,
		Dnn:           smPolicy.PolicyContext.Dnn,
		SliceInfo:     smPolicy.PolicyContext.SliceInfo,
		UeIpv4:        smPolicy.PolicyContext.Ipv4Address,
		UeIpv6:        smPolicy.PolicyContext.Ipv6AddressPrefix,
		MedComponents: map[string]models.MediaComponent{"1": medComp},
	}
	if ascReqData.NotifUri == "" {
		ascReqData.NotifUri = "http://127.0.0.1:7785/xcn-dedicated-bearer/v1/notifications"
	}
	if r.Dnn != "" {
		ascReqData.Dnn = r.Dnn
	}
	if r.Snssai != nil {
		ascReqData.SliceInfo = r.Snssai
	}
	return &models.AppSessionContext{AscReqData: ascReqData}
}

func xcnAppSessionMatches(appSession *pcf_context.AppSessionData) bool {
	return appSession != nil &&
		appSession.AppSessionContext != nil &&
		appSession.AppSessionContext.AscReqData != nil &&
		appSession.AppSessionContext.AscReqData.AfAppId == xcnDedicatedBearerAfAppID
}

func xcnAppSessionFlowDescriptions(appSession *pcf_context.AppSessionData) []string {
	var flows []string
	if !xcnAppSessionMatches(appSession) {
		return flows
	}
	for _, medComp := range appSession.AppSessionContext.AscReqData.MedComponents {
		for _, medSubComp := range medComp.MedSubComps {
			flows = append(flows, medSubComp.FDescs...)
		}
	}
	return flows
}

func sortedStrings(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func xcnWriteError(c *gin.Context, status int, detail string) {
	c.Set(sbi.IN_PB_DETAILS_CTX_STR, detail)
	c.JSON(status, models.ProblemDetails{
		Title:  http.StatusText(status),
		Status: int32(status),
		Detail: detail,
		Cause:  strings.ToUpper(strings.ReplaceAll(http.StatusText(status), " ", "_")),
	})
}

func (r XcnDedicatedBearerRequest) String() string {
	return "supi=" + r.Supi +
		",pduSessionId=" + strconv.Itoa(int(r.PduSessionID)) +
		",ueIp=" + r.UeIP +
		",ngapId=" + strconv.FormatInt(r.NgapID, 10) +
		",amfUeNgapId=" + strconv.FormatInt(r.AmfUeNgapID, 10) +
		",ranUeNgapId=" + strconv.FormatInt(r.RanUeNgapID, 10)
}
