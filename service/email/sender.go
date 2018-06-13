package email

import (
	"fmt"
	"github.com/nicksnyder/go-i18n/i18n"
	"github.com/ninjadotorg/handshake-exchange/common"
	"github.com/shopspring/decimal"
	"os"
)

func SendOfferBuyingActiveEmail(language string, emailAddress string, currency string, price string, priceCurrency string) error {
	T, _ := i18n.Tfunc(language)

	subject := T("email_offer_buying_active_subject", map[string]string{
		"Currency": currency,
	})

	priceNum, _ := decimal.NewFromString(price)
	priceStr := ""
	if priceNum.GreaterThan(common.Zero) {
		priceStr = fmt.Sprintf("for %s %s", price, priceCurrency)
	}
	data := struct {
		Currency string
		PriceStr string
	}{
		Currency: currency,
		PriceStr: priceStr,
	}

	return SendSystemEmailWithTemplate(
		"",
		emailAddress,
		language,
		subject,
		OfferBuyingActive,
		data)
}

func SendOfferSellingActiveEmail(language, emailAddress string, currency string, price string, priceCurrency string) error {
	T, _ := i18n.Tfunc(language)

	subject := T("email_offer_selling_active_subject", map[string]string{
		"Currency": currency,
	})

	priceNum, _ := decimal.NewFromString(price)
	priceStr := ""
	if priceNum.GreaterThan(common.Zero) {
		priceStr = fmt.Sprintf("for %s %s", price, priceCurrency)
	}
	data := struct {
		Currency string
		PriceStr string
	}{
		Currency: currency,
		PriceStr: priceStr,
	}

	return SendSystemEmailWithTemplate(
		"",
		emailAddress,
		language,
		subject,
		OfferSellingActive,
		data)
}

func SendOfferClosedEmail(language, emailAddress string) error {
	T, _ := i18n.Tfunc(language)

	subject := T("email_offer_closed_subject")
	host := os.Getenv("FRONTEND_HOST")
	data := struct {
		CreateOfferUrl string
	}{
		CreateOfferUrl: fmt.Sprintf("%s/create", host),
	}

	return SendSystemEmailWithTemplate(
		"",
		emailAddress,
		language,
		subject,
		OfferClosed,
		data)
}

func SendOfferMakerShakeEmail(language string, emailAddress string, username string,
	amount string, currency string, price string, fiatCurrency string) error {
	T, _ := i18n.Tfunc(language)

	subject := T("email_offer_maker_shake_subject", map[string]string{
		"Currency": currency,
	})

	data := struct {
		Amount       string
		Currency     string
		Price        string
		FiatCurrency string
		ToUsername   string
	}{
		Amount:       amount,
		Currency:     currency,
		Price:        price,
		FiatCurrency: fiatCurrency,
		ToUsername:   username,
	}

	return SendSystemEmailWithTemplate(
		"",
		emailAddress,
		language,
		subject,
		OfferMakerShake,
		data)
}

func SendOfferTakerShakeEmail(language string, emailAddress string, username string,
	amount string, currency string, price string, fiatCurrency string) error {
	T, _ := i18n.Tfunc(language)

	subject := T("email_offer_taker_shake_subject", map[string]string{
		"Currency": currency,
	})

	data := struct {
		Amount       string
		Currency     string
		Price        string
		FiatCurrency string
		Username     string
	}{
		Amount:       amount,
		Currency:     currency,
		Price:        price,
		FiatCurrency: fiatCurrency,
		Username:     username,
	}

	return SendSystemEmailWithTemplate(
		"",
		emailAddress,
		language,
		subject,
		OfferTakerShake,
		data)
}

func SendOfferMakerRejectEmail(language string, emailAddress string, username string) error {
	T, _ := i18n.Tfunc(language)

	subject := T("email_offer_maker_rejected_subject", map[string]string{
		"ToUsername": username,
	})

	data := struct {
		ToUsername string
	}{
		ToUsername: username,
	}

	return SendSystemEmailWithTemplate(
		"",
		emailAddress,
		language,
		subject,
		OfferMakerRejected,
		data)
}

func SendOfferTakerRejectEmail(language string, emailAddress string, username string) error {
	T, _ := i18n.Tfunc(language)

	subject := T("email_offer_taker_rejected_subject", map[string]string{
		"ToUsername": username,
	})

	data := struct {
		Username string
	}{
		Username: username,
	}

	return SendSystemEmailWithTemplate(
		"",
		emailAddress,
		language,
		subject,
		OfferTakerRejected,
		data)
}

func SendOfferCompleteEmail(language string, emailAddress string,
	amount string, currency string, username string) error {
	if emailAddress == "" {
		return nil
	}
	T, _ := i18n.Tfunc(language)

	subject := T("email_offer_completed_subject", map[string]string{
		"Currency": currency,
	})

	data := struct {
		Amount   string
		Currency string
		Username string
	}{
		Amount:   amount,
		Currency: currency,
		Username: username,
	}

	return SendSystemEmailWithTemplate(
		"",
		emailAddress,
		language,
		subject,
		OfferCompleted,
		data)
}

func SendOfferWithdrawEmail(language string, emailAddress string,
	amount string, currency string) error {
	if emailAddress == "" {
		return nil
	}
	T, _ := i18n.Tfunc(language)

	subject := T("email_offer_withdraw_subject", map[string]string{
		"Currency": currency,
	})

	data := struct {
		Amount   string
		Currency string
	}{
		Amount:   amount,
		Currency: currency,
	}

	return SendSystemEmailWithTemplate(
		"",
		emailAddress,
		language,
		subject,
		OfferWithdraw,
		data)
}

func SendOrderInstantCCSuccessEmail(language string, emailAddress string, amount string, currency string) error {
	T, _ := i18n.Tfunc(language)

	subject := T("email_order_instant_cc_success_subject")

	data := struct {
		Name     string
		Currency string
		Amount   string
	}{
		Name:     emailAddress,
		Currency: currency,
		Amount:   amount,
	}

	return SendSystemEmailWithTemplate(
		"",
		emailAddress,
		language,
		subject,
		OrderInstantCCSuccess,
		data)
}

func SendOfferStoreItemAddedEmail(language string, emailAddress string, sellAmount string, buyAmount string, currency string) error {
	T, _ := i18n.Tfunc(language)

	subject := T("email_offer_store_item_added")

	data := struct {
		Currency   string
		SellAmount string
		BuyAmount  string
	}{
		Currency:   currency,
		SellAmount: sellAmount,
		BuyAmount:  buyAmount,
	}

	return SendSystemEmailWithTemplate(
		"",
		emailAddress,
		language,
		subject,
		OfferStoreItemAdded,
		data)
}

func SendOfferStoreItemRemovedEmail(language string, emailAddress string) error {
	T, _ := i18n.Tfunc(language)

	subject := T("email_offer_store_item_removed")

	host := os.Getenv("FRONTEND_HOST")
	data := struct {
		Url string
	}{
		Url: fmt.Sprintf("%s/create", host),
	}

	return SendSystemEmailWithTemplate(
		"",
		emailAddress,
		language,
		subject,
		OfferStoreItemRemoved,
		data)
}

func SendOfferStoreMakerSellShakeEmail(language string, emailAddress string, amount string, currency string,
	fiatAmount string, fiatCurrency string, username string) error {
	if emailAddress == "" {
		return nil
	}
	T, _ := i18n.Tfunc(language)

	subject := T("email_offer_store_maker_sell_shake", map[string]string{
		"Currency": currency,
	})

	data := struct {
		Amount       string
		Currency     string
		FiatAmount   string
		FiatCurrency string
		Username     string
	}{
		Amount:       amount,
		Currency:     currency,
		FiatAmount:   fiatAmount,
		FiatCurrency: fiatCurrency,
		Username:     username,
	}

	return SendSystemEmailWithTemplate(
		"",
		emailAddress,
		language,
		subject,
		OfferStoreMakerSellShake,
		data)
}

func SendOfferStoreMakerBuyShakeEmail(language string, emailAddress string, amount string, currency string,
	fiatAmount string, fiatCurrency string, username string) error {
	if emailAddress == "" {
		return nil
	}

	T, _ := i18n.Tfunc(language)

	subject := T("email_offer_store_maker_buy_shake", map[string]string{
		"Currency": currency,
	})

	data := struct {
		Amount       string
		Currency     string
		FiatAmount   string
		FiatCurrency string
		Username     string
	}{
		Amount:       amount,
		Currency:     currency,
		FiatAmount:   fiatAmount,
		FiatCurrency: fiatCurrency,
		Username:     username,
	}

	return SendSystemEmailWithTemplate(
		"",
		emailAddress,
		language,
		subject,
		OfferStoreMakerBuyShake,
		data)
}

func SendOfferStoreTakerSellShakeEmail(language string, emailAddress string, amount string, currency string,
	fiatAmount string, fiatCurrency string, username string) error {
	if emailAddress == "" {
		return nil
	}
	T, _ := i18n.Tfunc(language)

	subject := T("email_offer_store_taker_sell_shake")

	data := struct {
		Amount       string
		Currency     string
		FiatAmount   string
		FiatCurrency string
		Username     string
	}{
		Amount:       amount,
		Currency:     currency,
		FiatAmount:   fiatAmount,
		FiatCurrency: fiatCurrency,
		Username:     username,
	}

	return SendSystemEmailWithTemplate(
		"",
		emailAddress,
		language,
		subject,
		OfferStoreTakerSellShake,
		data)
}

func SendOfferStoreTakerBuyShakeEmail(language string, emailAddress string, amount string, currency string,
	fiatAmount string, fiatCurrency string, username string) error {
	if emailAddress == "" {
		return nil
	}
	T, _ := i18n.Tfunc(language)

	subject := T("email_offer_store_taker_buy_shake")

	data := struct {
		Amount       string
		Currency     string
		FiatAmount   string
		FiatCurrency string
		Username     string
	}{
		Amount:       amount,
		Currency:     currency,
		FiatAmount:   fiatAmount,
		FiatCurrency: fiatCurrency,
		Username:     username,
	}

	return SendSystemEmailWithTemplate(
		"",
		emailAddress,
		language,
		subject,
		OfferStoreTakerBuyShake,
		data)
}

func SendOfferStoreAcceptEmail(language string, emailAddress string, amount string, currency string, username string) error {
	T, _ := i18n.Tfunc(language)

	subject := T("email_offer_store_accept", map[string]string{
		"Currency": currency,
	})

	data := struct {
		Amount   string
		Currency string
		Username string
	}{
		Amount:   amount,
		Currency: currency,
		Username: username,
	}

	return SendSystemEmailWithTemplate(
		"",
		emailAddress,
		language,
		subject,
		OfferStoreAccept,
		data)
}

func SendOfferStoreMakerRejectEmail(language string, emailAddress string, username string) error {
	if emailAddress == "" {
		return nil
	}
	T, _ := i18n.Tfunc(language)

	subject := T("email_offer_store_maker_reject", map[string]string{
		"Username": username,
	})

	host := os.Getenv("FRONTEND_HOST")
	data := struct {
		Url      string
		Username string
	}{
		Url:      fmt.Sprintf("%s/create", host),
		Username: username,
	}

	return SendSystemEmailWithTemplate(
		"",
		emailAddress,
		language,
		subject,
		OfferStoreMakerReject,
		data)
}

func SendOfferStoreTakerRejectEmail(language string, emailAddress string, username string) error {
	if emailAddress == "" {
		return nil
	}
	T, _ := i18n.Tfunc(language)

	subject := T("email_offer_store_taker_reject", map[string]string{
		"Username": username,
	})

	host := os.Getenv("FRONTEND_HOST")
	data := struct {
		Url      string
		Username string
	}{
		Url:      fmt.Sprintf("%s/discover", host),
		Username: username,
	}

	return SendSystemEmailWithTemplate(
		"",
		emailAddress,
		language,
		subject,
		OfferStoreTakerReject,
		data)
}
