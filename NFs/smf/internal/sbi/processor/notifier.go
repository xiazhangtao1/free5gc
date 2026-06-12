package processor

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/free5gc/openapi"
	"github.com/free5gc/openapi/models"
	"github.com/free5gc/openapi/smf/EventExposure"
	smf_context "github.com/free5gc/smf/internal/context"
	"github.com/free5gc/smf/internal/logger"
	"github.com/free5gc/util/metrics/sbi"
)

func (p *Processor) HandleChargingNotification(
	c *gin.Context,
	chargingNotifyRequest models.ChargingNotifyRequest,
	smContextRef string,
) {
	logger.ChargingLog.Info("Handle Charging Notification")

	problemDetails := p.chargingNotificationProcedure(chargingNotifyRequest, smContextRef)
	if problemDetails == nil {
		c.Status(http.StatusNoContent)
		return
	}
	c.Set(sbi.IN_PB_DETAILS_CTX_STR, problemDetails.Cause)
	c.JSON(int(problemDetails.Status), problemDetails)
}

// While receive Charging Notification from CHF, SMF will send Charging Information to CHF and update UPF
// The Charging Notification will be sent when CHF found the changes of the quota file.
func (p *Processor) chargingNotificationProcedure(
	req models.ChargingNotifyRequest, smContextRef string,
) *models.ProblemDetails {
	if smContext := smf_context.GetSMContextByRef(smContextRef); smContext != nil {
		smContext.SMLock.Lock()
		defer smContext.SMLock.Unlock()
		upfUrrMap := make(map[string][]*smf_context.URR)
		for _, reauthorizeDetail := range req.ReauthorizationDetails {
			rg := reauthorizeDetail.RatingGroup
			logger.ChargingLog.Infof("Force update charging information for rating group %d", rg)
			for upfId, entries := range smContext.UrrTable {
				for urrID, entry := range entries {
					chgInfo := smContext.ChargingInfo[urrID]
					if chgInfo.RatingGroup == rg ||
						chgInfo.ChargingLevel == smf_context.PduSessionCharging {
						logger.ChargingLog.Tracef("Query URR (%d) for Rating Group (%d)", urrID, rg)
						upfUrrMap[upfId] = append(upfUrrMap[upfId], entry.Rule)
					}
				}
			}
		}
		for upfId, urrList := range upfUrrMap {
			upf := smf_context.GetUpfById(upfId)
			if upf == nil {
				logger.ChargingLog.Warnf("Cound not find upf %s", upfId)
				continue
			}
			QueryReport(smContext, upf, urrList, models.ChfConvergedChargingTriggerType_FORCED_REAUTHORISATION)
		}
		p.ReportUsageAndUpdateQuota(smContext)
	} else {
		detail := fmt.Sprintf("SM Context [%s] Not Found ", smContextRef)
		return openapi.ProblemDetailsDataNotFound(detail)
	}

	return nil
}

func (p *Processor) HandleSMPolicyUpdateNotify(
	c *gin.Context,
	request models.SmPolicyNotification,
	smContextRef string,
) {
	logger.PduSessLog.Infoln("In HandleSMPolicyUpdateNotify")
	decision := request.SmPolicyDecision
	smContext := smf_context.GetSMContextByRef(smContextRef)

	if smContext == nil {
		logger.PduSessLog.Errorf("SMContext[%s] not found", smContextRef)
		c.Status(http.StatusBadRequest)
		return
	}

	smContext.SMLock.Lock()
	defer smContext.SMLock.Unlock()

	smContext.CheckState(smf_context.Active)
	// Wait till the state becomes Active again
	// TODO: implement waiting in concurrent architecture

	smContext.SetState(smf_context.ModificationPending)

	// Update SessionRule from decision
	if err := smContext.ApplySessionRules(decision); err != nil {
		// TODO: Fill the error body
		smContext.Log.Errorf("SMPolicyUpdateNotify err: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}

	// TODO: Response data type -
	// [200 OK] UeCampingRep
	// [200 OK] array(PartialSuccessReport)
	// [400 Bad Request] ErrorReport
	if err := smContext.ApplyPccRules(decision); err != nil {
		smContext.Log.Errorf("apply sm policy decision error: %+v", err)
		// TODO: Fill the error body
		c.Status(http.StatusBadRequest)
		return
	}

	smContext.SendUpPathChgNotification("EARLY", SendUpPathChgEventExposureNotification)

	ActivateUPFSession(smContext, nil)

	smContext.SendUpPathChgNotification("LATE", SendUpPathChgEventExposureNotification)

	smContext.PostRemoveDataPath()

	p.notifyPolicyTriggeredPduSessionModification(smContext)

	c.Status(http.StatusNoContent)
}

func (p *Processor) notifyPolicyTriggeredPduSessionModification(smContext *smf_context.SMContext) {
	if !hasPendingQosFlowForUe(smContext) {
		smContext.Log.Debugln("No pending QoS flow for policy-triggered PDU Session modification")
		return
	}

	n1Pdu, err := smf_context.BuildGSMPDUSessionModificationCommand(smContext)
	if err != nil {
		smContext.Log.Errorf("Build GSM PDU Session Modification Command failed: %v", err)
		return
	}

	n2Pdu, err := smf_context.BuildPDUSessionResourceModifyRequestTransfer(smContext)
	if err != nil {
		smContext.Log.Errorf("Build PDU Session Resource Modify Request Transfer failed: %v", err)
		return
	}

	n1n2Request := models.N1N2MessageTransferRequest{
		BinaryDataN1Message:     n1Pdu,
		BinaryDataN2Information: n2Pdu,
		JsonData: &models.N1N2MessageTransferReqData{
			PduSessionId: smContext.PDUSessionID,
			N1MessageContainer: &models.N1MessageContainer{
				N1MessageClass:   "SM",
				N1MessageContent: &models.RefToBinaryData{ContentId: "GSM_NAS"},
			},
			N2InfoContainer: &models.N2InfoContainer{
				N2InformationClass: models.N2InformationClass_SM,
				SmInfo: &models.N2SmInformation{
					PduSessionId: smContext.PDUSessionID,
					N2InfoContent: &models.N2InfoContent{
						NgapIeType: models.AmfCommunicationNgapIeType_PDU_RES_MOD_REQ,
						NgapData: &models.RefToBinaryData{
							ContentId: "N2SmInformation",
						},
					},
					SNssai: smContext.SNssai,
				},
			},
		},
	}

	ctx, _, err := smf_context.GetSelf().GetTokenCtx(models.ServiceName_NAMF_COMM, models.NrfNfManagementNfType_AMF)
	if err != nil {
		smContext.Log.Warnf("Get NAMF_COMM context failed: %s", err)
		return
	}

	rspData, err := p.Consumer().
		N1N2MessageTransfer(ctx, smContext.Supi, n1n2Request, smContext.CommunicationClientApiPrefix)
	if err != nil || rspData == nil {
		logger.ConsumerLog.Warnf("N1N2MessageTransfer for policy-triggered PDU Session modification failed: %+v", err)
		return
	}

	if rspData.Cause == models.N1N2MessageTransferCause_N1_MSG_NOT_TRANSFERRED {
		smContext.Log.Warnf("%v", rspData.Cause)
	}
}

func hasPendingQosFlowForUe(smContext *smf_context.SMContext) bool {
	if len(smContext.PendingQosFlowReleases) > 0 {
		return true
	}
	for _, qos := range smContext.AdditonalQosFlows {
		if qos.State == smf_context.QoSFlowUnset ||
			qos.State == smf_context.QoSFlowToBeModify ||
			qos.State == smf_context.QoSFlowToBeRemove {
			return true
		}
	}
	return false
}

func SendUpPathChgEventExposureNotification(
	uri string, notification *models.NsmfEventExposureNotification,
) {
	configuration := EventExposure.NewConfiguration()
	client := EventExposure.NewAPIClient(configuration)
	request := &EventExposure.CreateIndividualSubcriptionMyNotificationPostRequest{
		NsmfEventExposureNotification: notification,
	}
	_, err := client.
		SubscriptionsCollectionApi.
		CreateIndividualSubcriptionMyNotificationPost(context.Background(), uri, request)

	switch err := err.(type) {
	case openapi.GenericOpenAPIError:
		logger.PduSessLog.Warnf("SMF Event Exposure Notification Error[%s]", err.Error())
	case error:
		logger.PduSessLog.Warnf("SMF Event Exposure Notification Failed[%s]", err.Error())
	case nil:
		logger.PduSessLog.Tracef("SMF Event Exposure Notification Success")
	default:
		logger.PduSessLog.Warnf("SMF Event Exposure Notification Unknown Error: %+v", err)
	}
}
