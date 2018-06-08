package bean

import (
	"encoding/json"
	"fmt"
	"github.com/shopspring/decimal"
	"strconv"
	"strings"
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
	Id               string `json:"id"`
	FeedType         string `json:"feed_type"`
	Type             string `json:"type"`
	Amount           string `json:"amount"`
	Currency         string `json:"currency"`
	FiatCurrency     string `json:"fiat_currency"`
	FiatAmount       string `json:"fiat_amount"`
	TotalAmount      string `json:"total_amount"`
	PhysicalItem     string `json:"physical_item"`
	PhysicalQuantity int64  `json:"physical_quantity"`
	Fee              string `json:"fee"`
	Reward           string `json:"reward"`
	Price            string `json:"price"`
	Percentage       string `json:"percentage"`
	FeePercentage    string `json:"fee_percentage"`
	RewardPercentage string `json:"reward_percentage"`
	ContactPhone     string `json:"contact_phone"`
	ContactInfo      string `json:"contact_info"`
	Email            string `json:"email"`
	Username         string `json:"username"`
	ToEmail          string `json:"to_email"`
	ToUsername       string `json:"to_username"`
	SystemAddress    string `json:"system_address"`
	Status           string `json:"status"`
	Success          int64  `json:"success"`
	Failed           int64  `json:"failed"`
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
	Id            string `json:"id"`
	FeedType      string `json:"feed_type"`
	Type          string `json:"type"`
	Amount        string `json:"amount"`
	Currency      string `json:"currency"`
	FiatCurrency  string `json:"fiat_currency"`
	FiatAmount    string `json:"fiat_amount"`
	FeePercentage string `json:"fee_percentage"`
	Status        string `json:"status"`
	Email         string `json:"email"`
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

	feedType := "exchange"
	if len(offer.Tags) > 0 {
		feedType = fmt.Sprintf("exchange_%s", offer.Tags[0])
	}
	solr.OfferFeedType = feedType
	if offer.PhysicalItem != "" {
		solr.TextSearch = strings.Split(offer.PhysicalItem, " ")
	}
	solr.OfferType = offer.Type

	percentage, _ := decimal.NewFromString(offer.Percentage)
	feePercentage, _ := decimal.NewFromString(offer.FeePercentage)
	rewardPercentage, _ := decimal.NewFromString(offer.RewardPercentage)
	feePercentage = feePercentage.Add(rewardPercentage)
	fee, _ := decimal.NewFromString(offer.Fee)
	reward, _ := decimal.NewFromString(offer.Reward)
	fee = fee.Add(reward)

	extraData := SolrOfferExtraData{
		Id:               offer.Id,
		FeedType:         feedType,
		Type:             offer.Type,
		Amount:           offer.Amount,
		TotalAmount:      offer.TotalAmount,
		Currency:         offer.Currency,
		FiatAmount:       offer.FiatAmount,
		FiatCurrency:     offer.FiatCurrency,
		Price:            offer.Price,
		PhysicalItem:     offer.PhysicalItem,
		PhysicalQuantity: offer.PhysicalQuantity,
		Fee:              fee.String(),
		Reward:           offer.Reward,
		FeePercentage:    feePercentage.Mul(decimal.NewFromFloat(100)).String(),
		RewardPercentage: rewardPercentage.Mul(decimal.NewFromFloat(100)).String(),
		Percentage:       percentage.Mul(decimal.NewFromFloat(100)).String(),
		ContactInfo:      offer.ContactInfo,
		ContactPhone:     offer.ContactPhone,
		Email:            offer.Email,
		Username:         offer.Username,
		ToEmail:          offer.ToEmail,
		ToUsername:       offer.ToUsername,
		SystemAddress:    offer.SystemAddress,
		Status:           offer.Status,
		Success:          offer.TransactionCount.Success,
		Failed:           offer.TransactionCount.Failed,
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

	feePercentage, _ := decimal.NewFromString(offer.FeePercentage)
	extraData := SolrInstantOfferExtraData{
		Id:            offer.Id,
		FeedType:      "instant",
		Type:          "buy",
		Amount:        offer.Amount,
		Currency:      offer.Currency,
		FiatAmount:    offer.FiatAmount,
		FiatCurrency:  offer.FiatCurrency,
		FeePercentage: feePercentage.Mul(decimal.NewFromFloat(100)).String(),
		Status:        offer.Status,
		Email:         offer.Email,
	}
	b, _ := json.Marshal(&extraData)
	solr.ExtraData = string(b)

	return
}

type SolrLogObject struct {
	Id             string `json:"id"`
	UID            string `json:"uid_s"`
	RequestMethod  string `json:"request_method_s"`
	RequestURL     string `json:"request_url_s"`
	RequestData    string `json:"request_data_s"`
	ResponseStatus int    `json:"response_status_i"`
	ResponseData   string `json:"response_data_s"`
	Date           string `json:"data_s"`
	UpdateAt       int64  `json:"update_at_i"`
}

var instantOfferStoreStatusMap = map[string]int{
	OFFER_STORE_ITEM_STATUS_CREATED: 0,
	OFFER_STORE_ITEM_STATUS_ACTIVE:  1,
}

type SolrOfferStoreExtraData struct {
	Id            string                                `json:"id"`
	FeedType      string                                `json:"feed_type"`
	Type          string                                `json:"type"`
	ItemFlags     map[string]bool                       `json:"item_flags"`
	Username      string                                `json:"username"`
	Email         string                                `json:"email"`
	ContactPhone  string                                `json:"contact_phone"`
	ContactInfo   string                                `json:"contact_info"`
	FiatCurrency  string                                `json:"fiat_currency"`
	Status        string                                `json:"status"`
	Success       int64                                 `json:"success"`
	Failed        int64                                 `json:"failed"`
	ItemSnapshots map[string]SolrOfferStoreItemSnapshot `json:"items"`
}

type SolrOfferStoreItemSnapshot struct {
	Currency       string `json:"currency"`
	SellAmountMin  string `json:"sell_amount_min"`
	SellAmount     string `json:"sell_amount"`
	SellBalance    string `json:"sell_balance"`
	SellPercentage string `json:"sell_percentage"`
	BuyAmountMin   string `json:"buy_amount_min"`
	BuyAmount      string `json:"buy_amount"`
	BuyBalance     string `json:"buy_balance"`
	BuyPercentage  string `json:"buy_percentage"`
	SystemAddress  string `json:"system_address"`
}

func NewSolrFromOfferStore(offer OfferStore) (solr SolrOfferObject) {
	solr.Id = fmt.Sprintf("exchange_%s", offer.Id)
	solr.Type = 2
	solr.State = 0
	solr.IsPrivate = 1
	solr.Status = instantOfferStoreStatusMap[offer.Status]
	solr.Hid = 0
	solr.ChainId = offer.ChainId
	userId, _ := strconv.Atoi(offer.UID)
	solr.InitUserId = userId
	solr.ShakeUserIds = make([]int, 0)
	solr.TextSearch = make([]string, 0)
	solr.InitAt = offer.CreatedAt.Unix()
	solr.LastUpdateAt = time.Now().UTC().Unix()

	solr.OfferFeedType = "offer_store"
	// Nothing now
	solr.OfferType = ""

	var items = map[string]SolrOfferStoreItemSnapshot{}
	for key, value := range offer.ItemSnapshots {
		sellPercentage, _ := decimal.NewFromString(value.SellPercentage)
		buyPercentage, _ := decimal.NewFromString(value.BuyPercentage)
		items[key] = SolrOfferStoreItemSnapshot{
			Currency:       value.Currency,
			SellAmountMin:  value.SellAmountMin,
			SellAmount:     value.SellAmount,
			SellBalance:    value.SellBalance,
			SellPercentage: sellPercentage.Mul(decimal.NewFromFloat(100)).String(),
			BuyAmountMin:   value.BuyAmountMin,
			BuyAmount:      value.BuyAmount,
			BuyBalance:     value.BuyBalance,
			BuyPercentage:  buyPercentage.Mul(decimal.NewFromFloat(100)).String(),
			SystemAddress:  value.SystemAddress,
		}
	}

	extraData := SolrOfferStoreExtraData{
		Id:            offer.Id,
		FeedType:      "offer_store",
		Type:          "",
		ItemFlags:     offer.ItemFlags,
		ContactInfo:   offer.ContactInfo,
		ContactPhone:  offer.ContactPhone,
		Email:         offer.Email,
		Username:      offer.Username,
		Status:        offer.Status,
		FiatCurrency:  offer.FiatCurrency,
		Success:       offer.TransactionCount.Success,
		Failed:        offer.TransactionCount.Failed,
		ItemSnapshots: items,
	}
	b, _ := json.Marshal(&extraData)
	solr.ExtraData = string(b)

	return
}
