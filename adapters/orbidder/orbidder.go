package orbidder

import (
	"encoding/json"
	"fmt"
	"github.com/mxmCherry/openrtb"
	"github.com/prebid/prebid-server/adapters"
	"github.com/prebid/prebid-server/errortypes"
	"github.com/prebid/prebid-server/openrtb_ext"
	"net/http"
)

type OrbidderAdapter struct {
	endpoint string
}

// MakeRequests makes the HTTP requests which should be made to fetch bids from orbidder.
func (rcv *OrbidderAdapter) MakeRequests(request *openrtb.BidRequest, reqInfo *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
	var errs []error
	var validImps []openrtb.Imp

	// check if imps exists, if not do not return error and do send request to orbidder
	if len(request.Imp) == 0 {
		return nil, []error{&errortypes.BadInput{
			Message: "No impressions in request",
		}}
	}

	// validate imps
	for _, imp := range request.Imp {
		if err := preprocess(&imp); err != nil {
			errs = append(errs, err)
			continue
		}
		validImps = append(validImps, imp)
	}

	if len(validImps) == 0 {
		return nil, errs
	}

	//set imp array to only valid imps
	request.Imp = validImps

	requestBodyJSON, err := json.Marshal(request)
	if err != nil {
		errs = append(errs, err)
		return nil, errs
	}

	headers := http.Header{}
	headers.Add("Content-Type", "application/json;charset=utf-8")
	headers.Add("Accept", "application/json")

	return []*adapters.RequestData{{
		Method:  "POST",
		Uri:     rcv.endpoint,
		Body:    requestBodyJSON,
		Headers: headers,
	}}, errs
}

func preprocess(imp *openrtb.Imp) error {
	var bidderExt adapters.ExtImpBidder
	if err := json.Unmarshal(imp.Ext, &bidderExt); err != nil {
		return &errortypes.BadInput{
			Message: err.Error(),
		}
	}

	var orbidderExt openrtb_ext.ExtImpOrbidder
	if err := json.Unmarshal(bidderExt.Bidder, &orbidderExt); err != nil {
		return &errortypes.BadInput{
			Message: "Wrong orbidder bidder ext: " + err.Error(),
		}
	}

	if orbidderExt.AccountId == "" {
		return &errortypes.BadInput{
			Message: "Wrong orbidder bidder ext, no account id",
		}
	}

	if orbidderExt.PlacementId == "" {
		return &errortypes.BadInput{
			Message: "Wrong orbidder bidder ext, no placement id",
		}
	}

	return nil
}

// MakeBids unpacks server response into Bids.
func (rcv OrbidderAdapter) MakeBids(internalRequest *openrtb.BidRequest, externalRequest *adapters.RequestData, response *adapters.ResponseData) (*adapters.BidderResponse, []error) {
	if response.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	if response.StatusCode >= http.StatusInternalServerError {
		return nil, []error{&errortypes.BadServerResponse{
			Message: fmt.Sprintf("Unexpected status code: %d. Dsp server internal error.", response.StatusCode),
		}}
	}

	if response.StatusCode >= http.StatusBadRequest {
		return nil, []error{&errortypes.BadInput{
			Message: fmt.Sprintf("Unexpected status code: %d. Bad request to dsp.", response.StatusCode),
		}}
	}

	if response.StatusCode != http.StatusOK {
		return nil, []error{&errortypes.BadServerResponse{
			Message: fmt.Sprintf("Unexpected status code: %d. Bad response from dsp.", response.StatusCode),
		}}
	}

	var bidResp openrtb.BidResponse
	if err := json.Unmarshal(response.Body, &bidResp); err != nil {
		return nil, []error{err}
	}

	bidResponse := adapters.NewBidderResponseWithBidsCapacity(5)

	for _, seatBid := range bidResp.SeatBid {
		for _, bid := range seatBid.Bid {
			bidResponse.Bids = append(bidResponse.Bids, &adapters.TypedBid{
				Bid:     &bid,
				BidType: openrtb_ext.BidTypeBanner,
			})
		}
	}
	return bidResponse, nil
}

func NewOrbidderBidder(endpoint string) *OrbidderAdapter {
	return &OrbidderAdapter{
		endpoint: endpoint,
	}
}
