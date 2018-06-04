package bean

import (
	"encoding/json"
	"fmt"
	"github.com/shopspring/decimal"
	"strconv"
	"time"
)

type SolrOfferObject struct {
	Id            string   `json:"id"`
	Type          int      `json:"type_i"`
	State         int      `json:"state_i"`
	Status        int      `json:"status_i"`
	Hid           int64    `json:"hid_l"`
	IsPrivate     int      `json:"is_private_i"`
	InitUserId    int      `json:"init_user_id_i"`
	ChainId       int64    `json:"chain_id_i"`
	ShakeUserIds  []int    `json:"shake_user_ids_is"`
	ShakeCount    int      `json:"shake_count_i"`
	ViewCount     int      `json:"view_count_i"`
	CommentCount  int      `json:"comment_count_i"`
	TextSearch    []string `json:"text_search_ss"`
	ExtraData     string   `json:"extra_data_s"`
	OfferFeedType string   `json:"offer_feed_type_s"`
	OfferType     string   `json:"offer_type_s"`
	Location      string   `json:"location_p"`
	InitAt        int64    `json:"init_at_i"`
	LastUpdateAt  int64    `json:"last_update_at_i"`
}

type SolrOfferExtraData struct {
	Id            string `json:"id"`
	FeedType      string `json:"feed_type"`
	Type          string `json:"type"`
	Amount        string `json:"amount"`
	Currency      string `json:"currency"`
	FiatCurrency  string `json:"fiat_currency"`
	FiatAmount    string `json:"fiat_amount"`
	TotalAmount   string `json:"total_amount"`
	Fee           string `json:"fee"`
	Reward        string `json:"reward"`
	Price         string `json:"price"`
	Percentage    string `json:"percentage"`
	ContactPhone  string `json:"contact_phone"`
	ContactInfo   string `json:"contact_info"`
	Email         string `json:"email"`
	SystemAddress string `json:"system_address"`
	Status        string `json:"status"`
	Success       int64  `json:"success"`
	Failed        int64  `json:"failed"`
}

var offerStatusMap = map[string]int{
	OFFER_STATUS_CREATED:     0,
	OFFER_STATUS_ACTIVE:      1,
	OFFER_STATUS_CLOSING:     2,
	OFFER_STATUS_CLOSED:      3,
	OFFER_STATUS_SHAKING:     4,
	OFFER_STATUS_SHAKE:       5,
	OFFER_STATUS_COMPLETING:  6,
	OFFER_STATUS_COMPLETED:   7,
	OFFER_STATUS_WITHDRAWING: 8,
	OFFER_STATUS_WITHDRAW:    9,
	OFFER_STATUS_REJECTING:   10,
	OFFER_STATUS_REJECTED:    11,
}

type SolrInstantOfferExtraData struct {
	Id           string `json:"id"`
	FeedType     string `json:"feed_type"`
	Type         string `json:"type"`
	Amount       string `json:"amount"`
	Currency     string `json:"currency"`
	FiatCurrency string `json:"fiat_currency"`
	FiatAmount   string `json:"fiat_amount"`
	Status       string `json:"status"`
	Email        string `json:"email"`
}

var instantOfferStatusMap = map[string]int{
	INSTANT_OFFER_STATUS_PROCESSING: 0,
	INSTANT_OFFER_STATUS_SUCCESS:    1,
	INSTANT_OFFER_STATUS_CANCELLED:  2,
}

func NewSolrFromOffer(offer Offer) (solr SolrOfferObject) {
	solr.Id = fmt.Sprintf("exchange_%s", offer.Id)
	solr.Type = 2
	if offer.Status == OFFER_STATUS_ACTIVE {
		solr.State = 1
		solr.IsPrivate = 0
	} else {
		solr.State = 0
		solr.IsPrivate = 1
	}
	solr.Status = offerStatusMap[offer.Status]
	solr.Hid = offer.Hid
	solr.ChainId = offer.ChainId
	userId, _ := strconv.Atoi(offer.UID)
	solr.InitUserId = userId
	if offer.ToUID != "" {
		userId, _ := strconv.Atoi(offer.ToUID)
		solr.ShakeUserIds = []int{userId}
	} else {
		solr.ShakeUserIds = make([]int, 0)
	}
	solr.TextSearch = make([]string, 0)
	solr.Location = fmt.Sprintf("%f,%f", offer.Latitude, offer.Longitude)
	solr.InitAt = offer.CreatedAt.Unix()
	solr.LastUpdateAt = time.Now().UTC().Unix()

	solr.OfferFeedType = "exchange"
	solr.OfferType = offer.Type

	percentage, _ := decimal.NewFromString(offer.Percentage)
	extraData := SolrOfferExtraData{
		Id:            offer.Id,
		FeedType:      "exchange",
		Type:          offer.Type,
		Amount:        offer.Amount,
		TotalAmount:   offer.TotalAmount,
		Currency:      offer.Currency,
		FiatAmount:    offer.FiatAmount,
		FiatCurrency:  offer.FiatCurrency,
		Price:         offer.Price,
		Fee:           offer.Fee,
		Reward:        offer.Reward,
		Percentage:    percentage.Mul(decimal.NewFromFloat(100)).String(),
		ContactInfo:   offer.ContactInfo,
		ContactPhone:  offer.ContactPhone,
		Email:         offer.Email,
		SystemAddress: offer.SystemAddress,
		Status:        offer.Status,
		Success:       offer.TransactionCount.Success,
		Failed:        offer.TransactionCount.Failed,
	}
	b, _ := json.Marshal(&extraData)
	solr.ExtraData = string(b)

	return
}

func NewSolrFromInstantOffer(offer InstantOffer) (solr SolrOfferObject) {
	solr.Id = fmt.Sprintf("exchange_%s", offer.Id)
	solr.Type = 2
	solr.State = 0
	solr.IsPrivate = 1
	solr.Status = instantOfferStatusMap[offer.Status]
	solr.Hid = 0
	solr.ChainId = offer.ChainId
	userId, _ := strconv.Atoi(offer.UID)
	solr.InitUserId = userId
	solr.ShakeUserIds = make([]int, 0)
	solr.TextSearch = make([]string, 0)
	solr.InitAt = offer.CreatedAt.Unix()
	solr.LastUpdateAt = time.Now().UTC().Unix()

	solr.OfferFeedType = "instant"
	solr.OfferType = "buy"

	extraData := SolrInstantOfferExtraData{
		Id:           offer.Id,
		FeedType:     "instant",
		Type:         "buy",
		Amount:       offer.Amount,
		Currency:     offer.Currency,
		FiatAmount:   offer.FiatAmount,
		FiatCurrency: offer.FiatCurrency,
		Status:       offer.Status,
		Email:        offer.Email,
	}
	b, _ := json.Marshal(&extraData)
	solr.ExtraData = string(b)

	return
}
