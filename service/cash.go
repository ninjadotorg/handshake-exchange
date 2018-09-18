package service

import (
	"fmt"
	"github.com/ninjadotorg/handshake-exchange/api_error"
	"github.com/ninjadotorg/handshake-exchange/bean"
	"github.com/ninjadotorg/handshake-exchange/common"
	"github.com/ninjadotorg/handshake-exchange/dao"
	"github.com/ninjadotorg/handshake-exchange/integration/chainso_service"
	"github.com/ninjadotorg/handshake-exchange/integration/coinbase_service"
	"github.com/ninjadotorg/handshake-exchange/integration/crypto_service"
	"github.com/ninjadotorg/handshake-exchange/integration/exchangecreditatm_service"
	"github.com/ninjadotorg/handshake-exchange/integration/solr_service"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"strconv"
	"strings"
	"time"
)

type CashService struct {
	dao     *dao.CashDao
	miscDao *dao.MiscDao
	userDao *dao.UserDao
}

func (s CashService) GetCredit(userId string) (credit bean.CashCredit, ce SimpleContextError) {
	creditTO := s.dao.GetCashCredit(userId)
	if ce.FeedDaoTransfer(api_error.GetDataFailed, creditTO) {
		return
	}

	if creditTO.Found {
		credit = creditTO.Object.(bean.CashCredit)
		creditItemsTO := s.dao.ListCashCreditItem(userId)
		if ce.FeedDaoTransfer(api_error.GetDataFailed, creditTO) {
			return
		}
		creditItemMap := map[string]bean.CreditItem{}
		for _, creditItem := range creditItemsTO.Objects {
			item := creditItem.(bean.CreditItem)
			creditItemMap[item.Currency] = item
		}
		credit.Items = creditItemMap
	} else {
		ce.NotFound = true
	}

	return
}

func (s CashService) AddCashCredit(userId string, body bean.CashCredit) (credit bean.CashCredit, ce SimpleContextError) {
	creditTO := s.dao.GetCashCredit(userId)

	if creditTO.Error != nil {
		ce.FeedDaoTransfer(api_error.GetDataFailed, creditTO)
		return
	}

	var err error
	if creditTO.Found {
		ce.SetStatusKey(api_error.CreditExists)
	} else {
		body.UID = userId
		credit = body
		credit.Status = bean.CREDIT_STATUS_ACTIVE
		err = s.dao.AddCashCredit(&credit)
		if err != nil {
			ce.SetError(api_error.AddDataFailed, err)
			return
		}
		ce.NotFound = false
	}

	return
}

func (s CashService) DeactivateCashCredit(userId string, currency string) (credit bean.CashCredit, ce SimpleContextError) {
	creditTO := s.dao.GetCashCredit(userId)
	if ce.FeedDaoTransfer(api_error.GetDataFailed, creditTO) {
		return
	}
	credit = creditTO.Object.(bean.CashCredit)

	creditItemTO := s.dao.GetCashCreditItem(userId, currency)
	if ce.FeedDaoTransfer(api_error.GetDataFailed, creditItemTO) {
		return
	}
	creditItem := creditItemTO.Object.(bean.CashCreditItem)

	percentage, _ := strconv.Atoi(creditItem.Percentage)
	poolTO := s.dao.GetCashCreditPool(currency, percentage)
	if ce.FeedDaoTransfer(api_error.GetDataFailed, poolTO) {
		return
	}
	pool := poolTO.Object.(bean.CashCreditPool)

	if creditItem.Status == bean.CREDIT_ITEM_STATUS_ACTIVE && creditItem.SubStatus != bean.CREDIT_ITEM_SUB_STATUS_TRANSFERRING && creditItem.LockedSale == false {
		creditItem.Status = bean.CREDIT_ITEM_STATUS_INACTIVE

		itemHistory := bean.CashCreditBalanceHistory{}
		itemHistory.ModifyType = bean.CREDIT_POOL_MODIFY_TYPE_CLOSE

		poolHistory := bean.CashCreditPoolBalanceHistory{
			ModifyType: bean.CREDIT_POOL_MODIFY_TYPE_CLOSE,
		}

		poolOrders := make([]bean.CashCreditPoolOrder, 0)
		poolOrdersTO := s.dao.ListCashCreditPoolOrderUser(creditItem.Currency, userId)
		if ce.FeedDaoTransfer(api_error.GetDataFailed, poolOrdersTO) {
			return
		}
		for _, poolOrderItem := range poolOrdersTO.Objects {
			poolOrders = append(poolOrders, poolOrderItem.(bean.CashCreditPoolOrder))
		}
		err := s.dao.RemoveCashCreditItem(&creditItem, &itemHistory, &pool, poolOrders, &poolHistory)
		if err != nil {
			ce.SetError(api_error.UpdateDataFailed, err)
			return
		}
		client := exchangecreditatm_service.ExchangeCreditAtmClient{}
		// Change is negative, so need to revert to Positive
		amount := common.StringToDecimal(itemHistory.Change).Neg()

		if currency == bean.ETH.Code {
			nonce := CreditServiceInst.GetInstantTransferNonce(&ce)
			txHash, _, onChainErr := client.ReleasePartialFund(userId, 2, amount, creditItem.UserAddress, nonce, false)
			if onChainErr != nil {
				fmt.Println(onChainErr)
			} else {
			}
			fmt.Println(txHash)
			nonce += 1
			// s.SetNonceToCache(nonce)
		} else {
			coinbaseTx, errWithdraw := coinbase_service.SendTransaction(creditItem.UserAddress, amount.String(), currency,
				fmt.Sprintf("Refund userId = %s", creditItem.UID), creditItem.UID)
			if errWithdraw != nil {
				fmt.Println(errWithdraw)
			} else {
			}
			fmt.Println(coinbaseTx)
		}
	} else {
		ce.SetStatusKey(api_error.CreditItemStatusInvalid)
	}

	return
}

func (s CashService) AddCashDeposit(userId string, body bean.CashCreditDepositInput) (deposit bean.CashCreditDeposit, ce SimpleContextError) {
	var err error

	creditTO := s.dao.GetCashCredit(userId)
	if ce.FeedDaoTransfer(api_error.GetDataFailed, creditTO) {
		return
	}
	credit := creditTO.Object.(bean.CashCredit)

	// Minimum amount
	amount, _ := decimal.NewFromString(body.Amount)
	if body.Currency == bean.ETH.Code {
		if amount.LessThan(bean.MIN_ETH) {
			ce.SetStatusKey(api_error.AmountIsTooSmall)
			return
		}
	}
	if body.Currency == bean.BTC.Code {
		if amount.LessThan(bean.MIN_BTC) {
			ce.SetStatusKey(api_error.AmountIsTooSmall)
			return
		}
	}
	if body.Currency == bean.BCH.Code {
		if amount.LessThan(bean.MIN_BCH) {
			ce.SetStatusKey(api_error.AmountIsTooSmall)
			return
		}
	}

	creditItemTO := s.dao.GetCashCreditItem(userId, body.Currency)
	var creditItem bean.CashCreditItem
	if creditItemTO.Error != nil {
		ce.FeedDaoTransfer(api_error.GetDataFailed, creditItemTO)
		return
	} else {
		pNum, _ := strconv.Atoi(body.Percentage)
		if pNum < 0 || pNum > 200 {
			ce.SetStatusKey(api_error.InvalidRequestBody)
			return
		}
		if !creditItemTO.Found {
			creditItem = bean.CashCreditItem{
				UID:         userId,
				Currency:    body.Currency,
				Status:      bean.CREDIT_ITEM_STATUS_CREATE,
				Percentage:  body.Percentage,
				UserAddress: body.UserAddress,
			}
			err = s.dao.AddCashCreditItem(&creditItem)
			if err != nil {
				ce.SetError(api_error.AddDataFailed, err)
				return
			}
		} else {
			creditItem = creditItemTO.Object.(bean.CashCreditItem)
			if creditItem.Status == bean.CREDIT_ITEM_STATUS_INACTIVE {
				//Reactivate
				creditItem.Status = bean.CREDIT_ITEM_STATUS_ACTIVE
				creditItem.Percentage = body.Percentage
				creditItem.UserAddress = body.UserAddress

				err = s.dao.UpdateCashCreditItem(&creditItem)
				if err != nil {
					ce.SetError(api_error.AddDataFailed, err)
					return
				}
			} else {
				if creditItem.Percentage != body.Percentage {
					ce.SetStatusKey(api_error.InvalidRequestBody)
					return
				}
			}

			if creditItem.Status == bean.CREDIT_ITEM_STATUS_CREATE || creditItem.SubStatus == bean.CREDIT_ITEM_SUB_STATUS_TRANSFERRING {
				ce.SetStatusKey(api_error.CreditItemStatusInvalid)
				return
			}
			creditItem.UserAddress = body.UserAddress
		}
	}

	deposit = bean.CashCreditDeposit{
		UID:        userId,
		ItemRef:    dao.GetCashCreditItemItemPath(userId, body.Currency),
		Status:     bean.CREDIT_DEPOSIT_STATUS_CREATED,
		Currency:   body.Currency,
		Amount:     body.Amount,
		Percentage: body.Percentage,
	}

	if body.Currency != bean.ETH.Code {
		resp, errCoinbase := coinbase_service.GenerateAddress(body.Currency)
		if errCoinbase != nil {
			ce.SetError(api_error.ExternalApiFailed, errCoinbase)
			return
		}
		deposit.SystemAddress = resp.Data.Address
	}

	err = s.dao.AddCashCreditDeposit(&creditItem, &deposit)
	if err != nil {
		ce.SetError(api_error.AddDataFailed, err)
		return
	}

	chainId, _ := strconv.Atoi(credit.ChainId)
	deposit.CreatedAt = time.Now().UTC()
	solr_service.UpdateObject(bean.NewSolrFromCashCreditDeposit(deposit, int64(chainId)))
	s.dao.UpdateNotificationCashCreditItem(creditItem)

	return
}

func (s CashService) AdCashTracking(userId string, body bean.CashCreditOnChainActionTrackingInput) (tracking bean.CashCreditOnChainActionTracking, ce SimpleContextError) {
	creditTO := s.dao.GetCashCredit(userId)
	if ce.FeedDaoTransfer(api_error.GetDataFailed, creditTO) {
		return
	}
	credit := creditTO.Object.(bean.CashCredit)

	depositTO := s.dao.GetCashCreditDeposit(body.Currency, body.Deposit)
	if ce.FeedDaoTransfer(api_error.GetDataFailed, depositTO) {
		return
	}
	deposit := depositTO.Object.(bean.CashCreditDeposit)

	itemTO := s.dao.GetCashCreditItem(userId, body.Currency)
	if ce.FeedDaoTransfer(api_error.GetDataFailed, itemTO) {
		return
	}
	item := itemTO.Object.(bean.CashCreditItem)

	tracking = bean.CashCreditOnChainActionTracking{
		UID:        userId,
		ItemRef:    deposit.ItemRef,
		DepositRef: dao.GetCashCreditDepositItemPath(body.Currency, deposit.Id),
		TxHash:     body.TxHash,
		Action:     body.Action,
		Reason:     body.Reason,
		Currency:   body.Currency,
	}

	item.SubStatus = bean.CREDIT_ITEM_SUB_STATUS_TRANSFERRING
	deposit.Status = bean.CREDIT_DEPOSIT_STATUS_TRANSFERRING

	s.dao.AddCashCreditOnChainActionTracking(&item, &deposit, &tracking)

	chainId, _ := strconv.Atoi(credit.ChainId)
	solr_service.UpdateObject(bean.NewSolrFromCashCreditDeposit(deposit, int64(chainId)))
	s.dao.UpdateNotificationCashCreditItem(item)

	return
}

func (s CashService) FinishCashTracking() (ce SimpleContextError) {
	trackingTO := s.dao.ListCashCreditOnChainActionTracking(bean.ETH.Code)
	if ce.FeedDaoTransfer(api_error.GetDataFailed, trackingTO) {
		return
	}
	for _, item := range trackingTO.Objects {
		trackingItem := item.(bean.CashCreditOnChainActionTracking)

		if trackingItem.TxHash != "" {
			amount := decimal.Zero
			isSuccess, isPending, amount, errChain := crypto_service.GetTransactionReceipt(trackingItem.TxHash, trackingItem.Currency)
			if errChain == nil {
				if isSuccess && !isPending && amount.GreaterThan(common.Zero) {
					trackingItem.Amount = amount.String()
					s.finishTrackingCashItem(trackingItem)
				}
			} else {
				ce.SetError(api_error.ExternalApiFailed, errChain)
			}
		} else {
			s.dao.RemoveCashCreditOnChainActionTracking(trackingItem)
		}
	}

	trackingTO = s.dao.ListCashCreditOnChainActionTracking(bean.BTC.Code)
	if ce.FeedDaoTransfer(api_error.GetDataFailed, trackingTO) {
		return
	}
	for _, item := range trackingTO.Objects {
		trackingItem := item.(bean.CashCreditOnChainActionTracking)

		if trackingItem.TxHash != "" {
			confirmation, errChain := chainso_service.GetConfirmations(trackingItem.TxHash)
			amount := decimal.Zero
			if errChain == nil {
				amount, errChain = chainso_service.GetAmount(trackingItem.TxHash)
			} else {
				ce.SetError(api_error.ExternalApiFailed, errChain)
			}

			fmt.Println(fmt.Sprintf("%s %s %s %s", trackingItem.Id, trackingItem.UID, trackingItem.TxHash, amount.String()))
			confirmationRequired := s.getConfirmationRange(amount)
			if errChain == nil {
				if confirmation >= confirmationRequired && amount.GreaterThan(common.Zero) {
					trackingItem.Amount = amount.String()
					s.finishTrackingCashItem(trackingItem)
				}
			} else {
				ce.SetError(api_error.ExternalApiFailed, errChain)
			}
		} else {
			s.dao.RemoveCashCreditOnChainActionTracking(trackingItem)
		}
	}

	trackingTO = s.dao.ListCashCreditOnChainActionTracking(bean.BCH.Code)
	if ce.FeedDaoTransfer(api_error.GetDataFailed, trackingTO) {
		return
	}
	for _, item := range trackingTO.Objects {
		trackingItem := item.(bean.CashCreditOnChainActionTracking)

		if trackingItem.TxHash != "" {
			confirmation, errChain := chainso_service.GetConfirmations(trackingItem.TxHash)
			amount := decimal.Zero
			if errChain == nil {
				amount, errChain = chainso_service.GetAmount(trackingItem.TxHash)
			} else {
				ce.SetError(api_error.ExternalApiFailed, errChain)
			}

			fmt.Println(fmt.Sprintf("%s %s %s %s", trackingItem.Id, trackingItem.UID, trackingItem.TxHash, amount.String()))
			confirmationRequired := s.getConfirmationRange(amount)
			if errChain == nil {
				if confirmation >= confirmationRequired && amount.GreaterThan(common.Zero) {
					trackingItem.Amount = amount.String()
					s.finishTrackingCashItem(trackingItem)
				}
			} else {
				ce.SetError(api_error.ExternalApiFailed, errChain)
			}
		} else {
			s.dao.RemoveCashCreditOnChainActionTracking(trackingItem)
		}
	}

	return
}

func (s CashService) GetCashCreditPoolPercentageByCache(currency string, amount decimal.Decimal) (int, error) {
	percentage := 0
	for percentage <= 200 {
		level := fmt.Sprintf("%03d", percentage)

		creditPoolTO := s.dao.GetCashCreditPoolCache(currency, level)
		if creditPoolTO.HasError() {
			return 0, creditPoolTO.Error
		}
		if creditPoolTO.Found {
			creditPool := creditPoolTO.Object.(bean.CreditPool)
			remainingBalance := common.StringToDecimal(creditPool.Balance).Sub(common.StringToDecimal(creditPool.CapturedBalance))
			if remainingBalance.GreaterThanOrEqual(amount) {
				return percentage, nil
			}
		}

		percentage += 1
	}

	return 0, errors.New("not enough")
}

func (s CashService) AddCashCreditTransaction(trans *bean.CashCreditTransaction) (ce SimpleContextError) {
	poolTO := s.dao.GetCashCreditPool(trans.Currency, int(common.StringToDecimal(trans.Percentage).IntPart()))
	if ce.FeedDaoTransfer(api_error.GetDataFailed, poolTO) {
		return
	}
	pool := poolTO.Object.(bean.CashCreditPool)

	percentage := int(common.StringToDecimal(trans.Percentage).IntPart())
	level := fmt.Sprintf("%03d", percentage)
	orderTO := s.dao.ListCashCreditPoolOrder(trans.Currency, level)

	if ce.FeedDaoTransfer(api_error.GetDataFailed, orderTO) {
		return
	}
	amount := common.StringToDecimal(trans.Amount)
	selectedOrders := make([]bean.CashCreditPoolOrder, 0)

	userTransMap := map[string]*bean.CashCreditTransaction{}
	userTransList := make([]*bean.CashCreditTransaction, 0)

	needBreak := false
	for _, item := range orderTO.Objects {
		order := item.(bean.CashCreditPoolOrder)
		orderBalance := common.StringToDecimal(order.Balance)
		orderAmountSub := orderBalance.Sub(common.StringToDecimal(order.CapturedBalance))

		if !order.CapturedFull {
			var capturedAmount decimal.Decimal
			sub := amount.Sub(orderAmountSub)

			if sub.LessThan(common.Zero) {
				capturedAmount = amount
				needBreak = true
			} else {
				capturedAmount = orderAmountSub
				order.CapturedFull = true

				// out of amount, stop
				if sub.Equal(common.Zero) {
					needBreak = true
				} else {
					amount = amount.Sub(orderAmountSub)
				}
			}
			order.CapturedAmount = capturedAmount
			selectedOrders = append(selectedOrders, order)

			if userTrans, ok := userTransMap[order.UID]; ok {
				transAmount := common.StringToDecimal(userTrans.Amount)
				transAmount = transAmount.Add(capturedAmount)
				userTrans.Amount = transAmount.String()

				userTrans.OrderInfoRefs = append(userTrans.OrderInfoRefs, bean.CashCreditOrderInfoRef{
					OrderRef: dao.GetCreditPoolItemOrderItemPath(trans.Currency, level, order.Id),
					Amount:   capturedAmount.String(),
				})

				userTransMap[order.UID] = userTrans
			} else {
				userTrans = &bean.CashCreditTransaction{}
				userTrans.UID = order.UID
				userTrans.ToUID = trans.ToUID
				userTrans.Status = bean.CREDIT_TRANSACTION_STATUS_CREATE
				userTrans.Currency = trans.Currency
				userTrans.Percentage = trans.Percentage
				userTrans.OfferRef = trans.OfferRef
				userTrans.Amount = capturedAmount.String()

				userTrans.OrderInfoRefs = append(userTrans.OrderInfoRefs, bean.CashCreditOrderInfoRef{
					OrderRef: dao.GetCashCreditPoolItemOrderItemPath(trans.Currency, level, order.Id),
					Amount:   capturedAmount.String(),
				})

				userTransMap[order.UID] = userTrans
			}
			trans.OrderInfoRefs = append(trans.OrderInfoRefs, bean.CashCreditOrderInfoRef{
				OrderRef: dao.GetCashCreditPoolItemOrderItemPath(trans.Currency, level, order.Id),
				Amount:   capturedAmount.String(),
			})

			if needBreak {
				break
			}
		}
	}

	trans.Status = bean.CREDIT_TRANSACTION_STATUS_CREATE
	var transUserId string
	for k, v := range userTransMap {
		trans.UIDs = append(trans.UIDs, k)
		userTransList = append(userTransList, v)

		transUserId = k
	}

	if len(selectedOrders) == 0 {
		ce.SetStatusKey(api_error.CreditPriceChanged)
	}

	err := s.dao.AddCashCreditTransaction(&pool, trans, userTransList, selectedOrders)
	if err != nil {
		if strings.Contains(err.Error(), "out of stock") {
			ce.SetStatusKey(api_error.CreditPriceChanged)
			return
		} else {
			ce.SetError(api_error.AddDataFailed, err)
			return
		}
	}

	creditTO := s.dao.GetCashCredit(transUserId)
	if ce.FeedDaoTransfer(api_error.GetDataFailed, creditTO) {
		return
	}
	credit := creditTO.Object.(bean.Credit)

	chainId, _ := strconv.Atoi(credit.ChainId)
	for _, userTrans := range userTransList {
		userTrans.CreatedAt = time.Now().UTC()
		solr_service.UpdateObject(bean.NewSolrFromCashCreditTransaction(*userTrans, int64(chainId)))
	}

	return
}

func (s CashService) ListPendingCashCreditTransaction(currency string) (trans []bean.CashCreditTransaction, ce SimpleContextError) {
	transTO := s.dao.ListPendingCashCreditTransaction(currency)
	if ce.FeedDaoTransfer(api_error.GetDataFailed, transTO) {
		return
	}
	for _, item := range transTO.Objects {
		transItem := item.(bean.CashCreditTransaction)
		trans = append(trans, transItem)
	}

	return
}

func (s CashService) FinishCashCreditTransaction(currency string, id string, offerRef string,
	revenue decimal.Decimal, fee decimal.Decimal) (ce SimpleContextError) {
	transTO := s.dao.GetCashCreditTransaction(currency, id)

	if ce.FeedDaoTransfer(api_error.GetDataFailed, transTO) {
		return
	}
	trans := transTO.Object.(bean.CashCreditTransaction)
	trans.OfferRef = offerRef
	trans.Status = bean.CREDIT_TRANSACTION_STATUS_SUCCESS
	trans.SubStatus = bean.CREDIT_TRANSACTION_SUB_STATUS_REVENUE_PROCESSED
	trans.Revenue = revenue.RoundBank(2).String()

	amount := common.StringToDecimal(trans.Amount)

	poolTO := s.dao.GetCashCreditPool(trans.Currency, int(common.StringToDecimal(trans.Percentage).IntPart()))
	if ce.FeedDaoTransfer(api_error.GetDataFailed, transTO) {
		return
	}

	pool := poolTO.Object.(bean.CashCreditPool)
	poolHistory := bean.CashCreditPoolBalanceHistory{
		ItemRef:    "",
		ModifyRef:  dao.GetCashCreditTransactionItemPath(currency, id),
		ModifyType: bean.CREDIT_POOL_MODIFY_TYPE_PURCHASE,
		Change:     amount.Neg().String(),
	}

	items := make([]bean.CashCreditItem, 0)
	itemHistories := make([]bean.CashCreditBalanceHistory, 0)
	transList := make([]*bean.CashCreditTransaction, 0)
	var transUID string
	for _, userId := range trans.UIDs {
		itemTO := s.dao.GetCashCreditItem(userId, trans.Currency)
		if ce.FeedDaoTransfer(api_error.GetDataFailed, itemTO) {
			return
		}
		item := itemTO.Object.(bean.CashCreditItem)
		items = append(items, item)

		userTransTO := s.dao.GetCashCreditTransactionUser(userId, trans.Currency, trans.Id)
		if ce.FeedDaoTransfer(api_error.GetDataFailed, userTransTO) {
			return
		}
		userTrans := userTransTO.Object.(bean.CashCreditTransaction)
		userAmount := common.StringToDecimal(userTrans.Amount)

		percentageAmount := userAmount.Div(amount)
		userFee := percentageAmount.Mul(fee)
		userRevenue := percentageAmount.Mul(revenue).Sub(userFee)

		userTrans.OfferRef = offerRef
		userTrans.Status = bean.CREDIT_TRANSACTION_STATUS_SUCCESS
		userTrans.SubStatus = bean.CREDIT_TRANSACTION_SUB_STATUS_REVENUE_PROCESSED
		userTrans.Fee = userFee.RoundBank(2).String()
		userTrans.Revenue = userRevenue.RoundBank(2).String()
		transList = append(transList, &userTrans)

		itemHistory := bean.CashCreditBalanceHistory{
			ItemRef:    dao.GetCreditItemItemPath(userId, trans.Currency),
			ModifyRef:  dao.GetCreditTransactionItemUserPath(userId, currency, userTrans.Id),
			ModifyType: bean.CREDIT_POOL_MODIFY_TYPE_PURCHASE,
			Change:     userAmount.Neg().String(),
		}
		itemHistories = append(itemHistories, itemHistory)

		transUID = userTrans.UID
	}

	orders := make([]bean.CashCreditPoolOrder, 0)
	for _, orderInfo := range trans.OrderInfoRefs {
		orderTO := s.dao.GetCashCreditPoolOrderByPath(orderInfo.OrderRef)
		if ce.FeedDaoTransfer(api_error.GetDataFailed, orderTO) {
			return
		}
		order := orderTO.Object.(bean.CashCreditPoolOrder)
		order.CapturedAmount = common.StringToDecimal(orderInfo.Amount)
		orders = append(orders, order)
	}

	err := s.dao.FinishCashCreditTransaction(&pool, poolHistory, items, itemHistories, orders, &trans, transList)
	if err != nil {
		ce.SetError(api_error.UpdateDataFailed, err)
		return
	}

	creditTO := s.dao.GetCashCredit(transUID)
	if !creditTO.HasError() {
		credit := creditTO.Object.(bean.Credit)
		chainId, _ := strconv.Atoi(credit.ChainId)
		for _, userTrans := range transList {
			solr_service.UpdateObject(bean.NewSolrFromCashCreditTransaction(*userTrans, int64(chainId)))
		}
	}

	return
}

func (s CashService) AddCashCreditWithdraw(userId string, body bean.CashCreditWithdraw) (withdraw bean.CashCreditWithdraw, ce SimpleContextError) {
	creditTO := s.dao.GetCashCredit(userId)

	if creditTO.Error != nil {
		ce.FeedDaoTransfer(api_error.GetDataFailed, creditTO)
		return
	}
	credit := creditTO.Object.(bean.CashCredit)
	body.UID = userId
	body.Status = bean.CREDIT_WITHDRAW_STATUS_CREATED

	revenue := common.StringToDecimal(credit.Revenue)
	withdrawAmount := common.StringToDecimal(body.Amount)
	if withdrawAmount.GreaterThan(revenue) {
		ce.SetStatusKey(api_error.InvalidAmount)
		return
	}

	err := s.dao.AddCashCreditWithdraw(&credit, &body)
	if err != nil {
		if strings.Contains(err.Error(), "invalid amount") {
			ce.SetStatusKey(api_error.InvalidAmount)
			return
		}
		ce.SetError(api_error.UpdateDataFailed, err)
		return
	}

	withdraw = body
	withdraw.CreatedAt = time.Now().UTC()
	chainId, _ := strconv.Atoi(credit.ChainId)
	solr_service.UpdateObject(bean.NewSolrFromCashCreditWithdraw(withdraw, int64(chainId)))

	return
}

func (s CashService) ProcesCashCreditWithdraw() (ce SimpleContextError) {
	withdrawTO := s.dao.ListCashCreditWithdraw()
	if ce.FeedDaoTransfer(api_error.GetDataFailed, withdrawTO) {
		return
	}

	var credit bean.CashCredit
	withdraws := make([]bean.CashCreditWithdraw, 0)
	for _, item := range withdrawTO.Objects {
		withdraw := item.(bean.CashCreditWithdraw)
		withdraw.Status = bean.CREDIT_WITHDRAW_STATUS_PROCESSING
		s.dao.UpdateProcessingCashWithdraw(withdraw)

		if credit.UID == "" {
			creditTO := s.dao.GetCashCredit(withdraw.UID)
			if creditTO.Error != nil {
				ce.FeedDaoTransfer(api_error.GetDataFailed, creditTO)
				return
			}
			credit = creditTO.Object.(bean.CashCredit)
		}

		chainId, _ := strconv.Atoi(credit.ChainId)
		solr_service.UpdateObject(bean.NewSolrFromCashCreditWithdraw(withdraw, int64(chainId)))

		withdraws = append(withdraws, withdraw)
	}

	//err := email.SendCashCreditWithdrawEmail(withdraws, os.Getenv("ATM_CREDIT_WITHDRAW_EMAIL"))
	//if err != nil {
	//	ce.SetError(api_error.ExternalApiFailed, err)
	//	return
	//}

	return
}

func (s CashService) SetupCashCreditPool() (ce SimpleContextError) {
	for _, currency := range []string{bean.BTC.Code, bean.ETH.Code, bean.BCH.Code} {
		level := 0
		for level <= 200 {
			pool := bean.CashCreditPool{
				Level:    fmt.Sprintf("%03d", level),
				Balance:  common.Zero.String(),
				Currency: currency,
			}
			err := s.dao.AddCashCreditPool(&pool)
			s.dao.SetCashCreditPoolCache(pool)
			if err != nil {
				ce.SetError(api_error.AddDataFailed, err)
			}
			level += 1
		}
	}

	return
}

func (s CashService) SetupCashCreditPoolCache() (ce SimpleContextError) {
	for _, currency := range []string{bean.BTC.Code, bean.ETH.Code, bean.BCH.Code} {
		poolTO := s.dao.ListCashCreditPool(currency)
		if !poolTO.HasError() {
			for _, item := range poolTO.Objects {
				creditPool := item.(bean.CashCreditPool)
				s.dao.SetCashCreditPoolCache(creditPool)
			}
		}
	}

	return
}

func (s CashService) SetCashNonceToCache(nonce uint64) {
}

func (s CashService) SyncCashCreditTransactionToSolr(currency string, id string) (trans bean.CashCreditTransaction, ce SimpleContextError) {
	transTO := s.dao.GetCashCreditTransaction(currency, id)
	if ce.FeedDaoTransfer(api_error.GetDataFailed, transTO) {
		return
	}
	trans = transTO.Object.(bean.CashCreditTransaction)
	for _, userId := range trans.UIDs {
		creditTO := s.dao.GetCashCredit(userId)
		credit := creditTO.Object.(bean.Credit)

		transUserTO := s.dao.GetCashCreditTransactionUser(userId, currency, id)
		transUser := transUserTO.Object.(bean.CashCreditTransaction)

		chainId, _ := strconv.Atoi(credit.ChainId)
		solr_service.UpdateObject(bean.NewSolrFromCashCreditTransaction(transUser, int64(chainId)))
	}

	return
}

func (s CashService) SyncCashCreditDepositToSolr(currency string, id string) (deposit bean.CashCreditDeposit, ce SimpleContextError) {
	depositTO := s.dao.GetCashCreditDeposit(currency, id)
	if ce.FeedDaoTransfer(api_error.GetDataFailed, depositTO) {
		return
	}
	deposit = depositTO.Object.(bean.CashCreditDeposit)
	creditTO := s.dao.GetCashCredit(deposit.UID)
	credit := creditTO.Object.(bean.Credit)
	chainId, _ := strconv.Atoi(credit.ChainId)
	solr_service.UpdateObject(bean.NewSolrFromCashCreditDeposit(deposit, int64(chainId)))

	return
}

func (s CashService) SyncCashCreditWithdrawToSolr(id string) (withdraw bean.CashCreditWithdraw, ce SimpleContextError) {
	withdrawTO := s.dao.GetCashCreditWithdraw(id)
	if ce.FeedDaoTransfer(api_error.GetDataFailed, withdrawTO) {
		return
	}
	withdraw = withdrawTO.Object.(bean.CashCreditWithdraw)
	creditTO := s.dao.GetCashCredit(withdraw.UID)
	credit := creditTO.Object.(bean.Credit)
	chainId, _ := strconv.Atoi(credit.ChainId)
	solr_service.UpdateObject(bean.NewSolrFromCashCreditWithdraw(withdraw, int64(chainId)))

	return
}

func (s CashService) finishTrackingCashItem(tracking bean.CashCreditOnChainActionTracking) error {
	var err error

	creditTO := s.dao.GetCashCredit(tracking.UID)
	if creditTO.HasError() {
		return creditTO.Error
	}
	credit := creditTO.Object.(bean.Credit)

	depositTO := s.dao.GetCashCreditDepositByPath(tracking.DepositRef)
	if depositTO.HasError() {
		return depositTO.Error
	}
	deposit := depositTO.Object.(bean.CashCreditDeposit)

	itemTO := s.dao.GetCashCreditItem(tracking.UID, tracking.Currency)
	if itemTO.HasError() {
		return itemTO.Error
	}
	item := itemTO.Object.(bean.CashCreditItem)

	if item.Status == bean.CREDIT_ITEM_STATUS_CREATE || item.Status == bean.CREDIT_ITEM_STATUS_INACTIVE {
		item.Status = bean.CREDIT_ITEM_STATUS_ACTIVE
	}
	item.SubStatus = bean.CREDIT_ITEM_SUB_STATUS_TRANSFERRED
	item.LastActionData = deposit
	deposit.Status = bean.CREDIT_DEPOSIT_STATUS_TRANSFERRED

	poolTO := s.dao.GetCashCreditPool(item.Currency, int(common.StringToDecimal(item.Percentage).IntPart()))
	if poolTO.HasError() {
		return poolTO.Error
	}
	pool := poolTO.Object.(bean.CashCreditPool)
	itemHistory := bean.CashCreditBalanceHistory{
		ItemRef:    tracking.ItemRef,
		ModifyRef:  tracking.DepositRef,
		ModifyType: tracking.Action,
	}
	poolHistory := bean.CashCreditPoolBalanceHistory{
		ItemRef:    tracking.ItemRef,
		ModifyRef:  tracking.DepositRef,
		ModifyType: tracking.Action,
	}
	poolOrder := bean.CashCreditPoolOrder{
		Id:         time.Now().UTC().Format("2006-01-02T15:04:05.000000000"),
		UID:        tracking.UID,
		DepositRef: tracking.DepositRef,
		Amount:     tracking.Amount,
		Balance:    tracking.Amount,
	}

	err = s.dao.FinishDepositCashCreditItem(&item, &deposit, &itemHistory, &pool, &poolOrder, &poolHistory, &tracking)
	if err != nil {
		return err
	}

	chainId, _ := strconv.Atoi(credit.ChainId)
	solr_service.UpdateObject(bean.NewSolrFromCashCreditDeposit(deposit, int64(chainId)))
	s.dao.UpdateNotificationCashCreditItem(item)

	return err
}

func (s CashService) finishFailedCashTrackingItem(tracking bean.CashCreditOnChainActionTracking) error {
	var err error

	depositTO := s.dao.GetCashCreditDepositByPath(tracking.DepositRef)
	if depositTO.HasError() {
		return depositTO.Error
	}
	deposit := depositTO.Object.(bean.CashCreditDeposit)

	itemTO := s.dao.GetCashCreditItem(tracking.UID, tracking.Currency)
	if itemTO.HasError() {
		return itemTO.Error
	}
	item := itemTO.Object.(bean.CashCreditItem)

	if item.Status == bean.CREDIT_ITEM_STATUS_CREATE || item.Status == bean.CREDIT_ITEM_STATUS_INACTIVE {
		item.Status = bean.CREDIT_ITEM_STATUS_INACTIVE
	}
	item.SubStatus = ""
	deposit.Status = bean.CREDIT_DEPOSIT_STATUS_FAILED

	s.dao.FinishFailedDepositCashCreditItem(&item, &deposit, &tracking)

	return err
}

func (s CashService) getConfirmationRange(amount decimal.Decimal) int {
	if amount.LessThan(decimal.NewFromFloat(0.5)) {
		return 1
	} else if amount.LessThan(decimal.NewFromFloat(1)) {
		return 3
	}

	return 6
}