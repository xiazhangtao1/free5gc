package consumer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/free5gc/openapi"
	"github.com/free5gc/openapi/amf/Communication"
	"github.com/free5gc/openapi/models"
	"github.com/free5gc/openapi/nrf/NFDiscovery"
	pcf_context "github.com/free5gc/pcf/internal/context"
	"github.com/free5gc/pcf/internal/logger"
	"github.com/free5gc/pcf/internal/util"
	"github.com/free5gc/pcf/pkg/factory"
	sbi_metrics "github.com/free5gc/util/metrics/sbi"
)

type namfService struct {
	consumer *Consumer

	nfComMu sync.RWMutex
	oamMu   sync.RWMutex

	nfComClients  map[string]*Communication.APIClient
	oamHTTPClient *http.Client
}

type OAMPduSession struct {
	PduSessionId string `json:"PduSessionId"`
	SmContextRef string `json:"SmContextRef"`
	Sst          string `json:"Sst"`
	Sd           string `json:"Sd"`
	Dnn          string `json:"Dnn"`
	AmfUeNgapId  int64  `json:"AmfUeNgapId"`
	RanUeNgapId  int64  `json:"RanUeNgapId"`
}

type OAMUEContext struct {
	AccessType  models.AccessType `json:"AccessType"`
	Supi        string            `json:"Supi"`
	Guti        string            `json:"Guti"`
	AmfUeNgapId int64             `json:"AmfUeNgapId"`
	RanUeNgapId int64             `json:"RanUeNgapId"`
	Mcc         string            `json:"Mcc"`
	Mnc         string            `json:"Mnc"`
	Tac         string            `json:"Tac"`
	PduSessions []OAMPduSession   `json:"PduSessions"`
	CmState     models.CmState    `json:"CmState"`
}

func (s *namfService) getNFCommunicationClient(uri string) *Communication.APIClient {
	if uri == "" {
		return nil
	}
	s.nfComMu.RLock()
	client, ok := s.nfComClients[uri]
	if ok {
		defer s.nfComMu.RUnlock()
		return client
	}

	configuration := Communication.NewConfiguration()
	configuration.SetBasePath(uri)
	configuration.SetMetrics(sbi_metrics.SbiMetricHook)
	client = Communication.NewAPIClient(configuration)

	s.nfComMu.RUnlock()
	s.nfComMu.Lock()
	defer s.nfComMu.Unlock()
	s.nfComClients[uri] = client
	return client
}

func (s *namfService) getOAMHTTPClient() *http.Client {
	s.oamMu.RLock()
	client := s.oamHTTPClient
	s.oamMu.RUnlock()
	if client != nil {
		return client
	}

	s.oamMu.Lock()
	defer s.oamMu.Unlock()
	if s.oamHTTPClient == nil {
		s.oamHTTPClient = &http.Client{}
	}
	return s.oamHTTPClient
}

func (s *namfService) GetRegisteredUEContextsFromOAM() ([]OAMUEContext, error) {
	targetNfType := models.NrfNfManagementNfType_AMF
	requesterNfType := models.NrfNfManagementNfType_PCF
	request := NFDiscovery.SearchNFInstancesRequest{}

	result, err := s.consumer.nnrfService.SendSearchNFInstances(
		s.consumer.Context().NrfUri,
		targetNfType,
		requesterNfType,
		request,
	)
	if err != nil {
		return nil, err
	}
	if result == nil || len(result.NfInstances) == 0 {
		return nil, fmt.Errorf("no AMF instances found")
	}

	var lastErr error
	for _, profile := range result.NfInstances {
		amfURI := util.SearchNFServiceUri(profile, models.ServiceName_NAMF_OAM, models.NfServiceStatus_REGISTERED)
		if amfURI == "" {
			continue
		}
		ueContexts, err := s.getRegisteredUEContextsFromOAMURI(amfURI)
		if err == nil {
			return ueContexts, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no AMF OAM service found")
}

func (s *namfService) getRegisteredUEContextsFromOAMURI(amfURI string) ([]OAMUEContext, error) {
	url := strings.TrimRight(amfURI, "/") + "/namf-oam/v1/registered-ue-context"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.getOAMHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("AMF OAM returned status %d", resp.StatusCode)
	}

	var ueContexts []OAMUEContext
	if err := json.NewDecoder(resp.Body).Decode(&ueContexts); err != nil {
		return nil, err
	}
	return ueContexts, nil
}

func (s *namfService) AmfStatusChangeSubscribe(amfUri string, guamiList []models.Guami) (
	problemDetails *models.ProblemDetails, err error,
) {
	logger.ConsumerLog.Debugf("PCF Subscribe to AMF status[%+v]", amfUri)
	pcfContext := s.consumer.pcf.Context()

	// Set client and set url
	client := s.getNFCommunicationClient(amfUri)

	subscriptionData := models.AmfCommunicationSubscriptionData{
		AmfStatusUri: fmt.Sprintf("%s"+factory.PcfCallbackResUriPrefix+"/amfstatus", pcfContext.GetIPv4Uri()),
		GuamiList:    guamiList,
	}
	amfStausChangeRequest := &Communication.AMFStatusChangeSubscribeRequest{}
	amfStausChangeRequest.SetAmfCommunicationSubscriptionData(subscriptionData)
	ctx, pd, err := pcfContext.GetTokenCtx(models.ServiceName_NAMF_COMM, models.NrfNfManagementNfType_AMF)
	if err != nil {
		return pd, err
	}
	res, localErr := client.SubscriptionsCollectionCollectionApi.AMFStatusChangeSubscribe(
		ctx, amfStausChangeRequest)

	if localErr == nil {
		locationHeader := res.Location
		logger.ConsumerLog.Debugf("location header: %+v", locationHeader)

		subscriptionID := locationHeader[strings.LastIndex(locationHeader, "/")+1:]
		amfStatusSubsData := pcf_context.AMFStatusSubscriptionData{
			AmfUri:       amfUri,
			AmfStatusUri: res.AmfCommunicationSubscriptionData.AmfStatusUri,
			GuamiList:    res.AmfCommunicationSubscriptionData.GuamiList,
		}
		pcfContext.NewAmfStatusSubscription(subscriptionID, amfStatusSubsData)
	}

	if genericErr, ok := localErr.(openapi.GenericOpenAPIError); ok {
		if problemDetails, ok := genericErr.Model().(models.ProblemDetails); ok {
			return &problemDetails, nil
		}

		logger.ConsumerLog.Errorf("openapi error: %+v", localErr)
		return nil, localErr
	}
	return nil, localErr
}
