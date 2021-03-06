package dao

import (
	"cloud.google.com/go/firestore"
	"context"
	"errors"
	"fmt"
	"github.com/ninjadotorg/handshake-exchange/bean"
	"github.com/ninjadotorg/handshake-exchange/common"
	"github.com/ninjadotorg/handshake-exchange/integration/firebase_service"
	"github.com/shopspring/decimal"
	"strings"
)

type CoinDao struct {
}

func (dao CoinDao) ListCoinCenter(country string) (t TransferObject) {
	ListObjects(GetCoinCenterCountryPath(country), &t, nil, snapshotToCoinCenter)
	return
}

func (dao CoinDao) ListCoinBank(country string) (t TransferObject) {
	ListObjects(GetCoinBankCountryPath(country), &t, nil, snapshotToCoinBank)
	return
}

func (dao CoinDao) GetCoinOrder(id string) (t TransferObject) {
	GetObject(GetCoinOrderItemPath(id), &t, snapshotToCoinOrder)
	return
}

func (dao CoinDao) GetCoinSellingOrder(id string) (t TransferObject) {
	GetObject(GetCoinSellingOrderItemPath(id), &t, snapshotToCoinSellingOrder)
	return
}

func (dao CoinDao) GetCoinOrderByPath(path string) (t TransferObject) {
	GetObject(path, &t, snapshotToCoinOrder)
	return
}

func (dao CoinDao) ListCoinOrders(status string, orderType string, refCode string, limit int, startAt interface{}) (t TransferObject) {
	ListPagingObjects(GetCoinOrderPath(), &t, limit, startAt, func(collRef *firestore.CollectionRef) firestore.Query {
		query := collRef.OrderBy("created_at", firestore.Desc)
		if refCode != "" {
			query = query.Where("ref_code", "==", refCode)
		} else {
			if status != "" {
				query = query.Where("status", "==", status)
			}
			if orderType != "" {
				query = query.Where("type", "==", orderType)
			}
		}

		return query
	}, snapshotToCoinOrder)

	return
}

func (dao CoinDao) ListCoinSellingOrders(status string, refCode string, limit int, startAt interface{}) (t TransferObject) {
	ListPagingObjects(GetCoinSellingOrderPath(), &t, limit, startAt, func(collRef *firestore.CollectionRef) firestore.Query {
		query := collRef.OrderBy("created_at", firestore.Desc)
		if refCode != "" {
			query = query.Where("ref_code", "==", refCode)
		} else {
			if status != "" {
				query = query.Where("status", "==", status)
			}
		}

		return query
	}, snapshotToCoinSellingOrder)

	return
}

func (dao CoinDao) AddCoinOrder(order *bean.CoinOrder) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Collection(GetCoinOrderPath()).NewDoc()
	order.Id = docRef.ID
	docUserRef := dbClient.Doc(GetCoinOrderUserItemPath(order.UID, order.Id))

	refCode := strings.ToLower(order.Id[:6])
	orderRefCode := bean.CoinOrderRefCode{
		RefCode:  refCode,
		OrderRef: GetCoinOrderItemPath(order.Id),
		Order:    order.Id,
		Duration: order.Duration,
	}
	docOrderRefRef := dbClient.Doc(GetCoinOrderRefCodeItemPath(refCode))
	order.RefCode = refCode

	docPoolRef := dbClient.Doc(GetCoinPoolItemPath(order.Currency))

	docUserLimitRef := dbClient.Doc(GetCoinUserLimitItemPath(order.UID))

	err := dbClient.RunTransaction(context.Background(), func(ctx context.Context, tx *firestore.Transaction) error {
		var txErr error

		poolDoc, txErr := tx.Get(docPoolRef)
		if txErr != nil {
			return txErr
		}
		usage, txErr := common.ConvertToDecimal(poolDoc, "usage")
		if txErr != nil {
			return txErr
		}
		limit, txErr := common.ConvertToDecimal(poolDoc, "limit")
		if txErr != nil {
			return txErr
		}
		amount := common.StringToDecimal(order.Amount)
		usage = usage.Add(amount)
		if usage.GreaterThan(limit) {
			return errors.New("out of stock")
		}

		userLimitDoc, txErr := tx.Get(docUserLimitRef)
		userUsage, txErr := common.ConvertToDecimal(userLimitDoc, "usage")
		if txErr != nil {
			return txErr
		}
		userLimit, txErr := common.ConvertToDecimal(userLimitDoc, "limit")
		if txErr != nil {
			return txErr
		}
		fiatAmount := common.StringToDecimal(order.FiatLocalAmount)
		userUsage = userUsage.Add(fiatAmount)
		if userUsage.GreaterThan(userLimit) {
			return errors.New("over limit")
		}

		txErr = tx.Set(docRef, order.GetAdd(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}
		txErr = tx.Set(docUserRef, order.GetAdd(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}
		if order.Type != bean.COIN_ORDER_TYPE_COD {
			txErr = tx.Set(docOrderRefRef, orderRefCode.GetAdd(), firestore.MergeAll)
			if txErr != nil {
				return txErr
			}
		}
		txErr = tx.Set(docPoolRef, bean.CoinPool{
			Usage: usage.String(),
		}.GetUpdate(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}

		txErr = tx.Set(docUserLimitRef, bean.CoinUserLimit{
			Usage: userUsage.String(),
		}.GetUpdate(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}

		return txErr
	})

	return err
}

func (dao CoinDao) AddCoinSellingOrder(order *bean.CoinSellingOrder) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Collection(GetCoinSellingOrderPath()).NewDoc()
	order.Id = docRef.ID
	docUserRef := dbClient.Doc(GetCoinSellingOrderUserItemPath(order.UID, order.Id))

	refCode := strings.ToLower(order.Id[:6])
	orderRefCode := bean.CoinOrderRefCode{
		RefCode:  refCode,
		OrderRef: GetCoinOrderItemPath(order.Id),
		Order:    order.Id,
		Duration: order.Duration,
	}
	docOrderRefRef := dbClient.Doc(GetCoinSellingOrderRefCodeItemPath(order.Address))
	order.RefCode = refCode

	addressTracking := bean.CoinAddressTracking{
		Address:  order.Address,
		Currency: order.Currency,
		OrderRef: GetCoinOrderItemPath(order.Id),
		Order:    order.Id,
	}
	docTrackingRef := dbClient.Doc(GetCoinAddressTrackingItemPath(order.Currency, order.Address))

	docPoolRef := dbClient.Doc(GetCoinSellingPoolItemPath(order.Currency))

	docUserLimitRef := dbClient.Doc(GetCoinSellingUserLimitItemPath(order.UID))

	err := dbClient.RunTransaction(context.Background(), func(ctx context.Context, tx *firestore.Transaction) error {
		var txErr error

		poolDoc, txErr := tx.Get(docPoolRef)
		if txErr != nil {
			return txErr
		}
		usage, txErr := common.ConvertToDecimal(poolDoc, "usage")
		if txErr != nil {
			return txErr
		}
		limit, txErr := common.ConvertToDecimal(poolDoc, "limit")
		if txErr != nil {
			return txErr
		}
		amount := common.StringToDecimal(order.Amount)
		usage = usage.Add(amount)
		if usage.GreaterThan(limit) {
			return errors.New("out of stock")
		}

		userLimitDoc, txErr := tx.Get(docUserLimitRef)
		userUsage, txErr := common.ConvertToDecimal(userLimitDoc, "usage")
		if txErr != nil {
			return txErr
		}
		userLimit, txErr := common.ConvertToDecimal(userLimitDoc, "limit")
		if txErr != nil {
			return txErr
		}
		fiatAmount := common.StringToDecimal(order.FiatLocalAmount)
		userUsage = userUsage.Add(fiatAmount)
		if userUsage.GreaterThan(userLimit) {
			return errors.New("over limit")
		}

		txErr = tx.Set(docRef, order.GetAdd(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}
		txErr = tx.Set(docUserRef, order.GetAdd(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}
		txErr = tx.Set(docOrderRefRef, orderRefCode.GetAdd(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}

		txErr = tx.Set(docPoolRef, bean.CoinPool{
			Usage: usage.String(),
		}.GetUpdate(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}

		txErr = tx.Set(docUserLimitRef, bean.CoinUserLimit{
			Usage: userUsage.String(),
		}.GetUpdate(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}

		if order.Currency != bean.ETH.Code {
			txErr = tx.Set(docTrackingRef, addressTracking.GetAdd())
			if txErr != nil {
				return txErr
			}
		}

		return txErr
	})

	return err
}

func (dao CoinDao) CancelCoinOrder(order *bean.CoinOrder) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinOrderItemPath(order.Id))
	docUserRef := dbClient.Doc(GetCoinOrderUserItemPath(order.UID, order.Id))
	docOrderRefRef := dbClient.Doc(GetCoinOrderRefCodeItemPath(order.Address))

	docPoolRef := dbClient.Doc(GetCoinPoolItemPath(order.Currency))
	docUserLimitRef := dbClient.Doc(GetCoinUserLimitItemPath(order.UID))

	err := dbClient.RunTransaction(context.Background(), func(ctx context.Context, tx *firestore.Transaction) error {
		var txErr error

		poolDoc, txErr := tx.Get(docPoolRef)
		if txErr != nil {
			return txErr
		}
		usage, txErr := common.ConvertToDecimal(poolDoc, "usage")
		if txErr != nil {
			return txErr
		}
		amount := common.StringToDecimal(order.Amount)
		usage = usage.Sub(amount)
		if usage.LessThan(common.Zero) {
			usage = common.Zero
		}

		userLimitDoc, txErr := tx.Get(docUserLimitRef)
		if txErr != nil {
			return txErr
		}
		userUsage, txErr := common.ConvertToDecimal(userLimitDoc, "usage")
		if txErr != nil {
			return txErr
		}
		fiatAmount := common.StringToDecimal(order.FiatLocalAmount)
		userUsage = userUsage.Sub(fiatAmount)
		if userUsage.LessThan(common.Zero) {
			userUsage = common.Zero
		}

		txErr = tx.Set(docRef, order.GetUpdate(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}
		txErr = tx.Set(docUserRef, order.GetUpdate(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}
		if order.Type != bean.COIN_ORDER_TYPE_COD {
			txErr = tx.Delete(docOrderRefRef)
			if txErr != nil {
				return txErr
			}
		}

		txErr = tx.Set(docPoolRef, bean.CoinPool{
			Usage: usage.String(),
		}.GetUpdate(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}

		txErr = tx.Set(docUserLimitRef, bean.CoinUserLimit{
			Usage: userUsage.String(),
		}.GetUpdate(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}

		return txErr
	})

	return err
}

func (dao CoinDao) CancelCoinSellingOrder(order *bean.CoinSellingOrder) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinSellingOrderItemPath(order.Id))
	docUserRef := dbClient.Doc(GetCoinSellingOrderUserItemPath(order.UID, order.Id))
	docOrderRefRef := dbClient.Doc(GetCoinSellingOrderRefCodeItemPath(order.RefCode))

	docPoolRef := dbClient.Doc(GetCoinSellingPoolItemPath(order.Currency))
	docUserLimitRef := dbClient.Doc(GetCoinSellingUserLimitItemPath(order.UID))

	err := dbClient.RunTransaction(context.Background(), func(ctx context.Context, tx *firestore.Transaction) error {
		var txErr error

		poolDoc, txErr := tx.Get(docPoolRef)
		if txErr != nil {
			return txErr
		}
		usage, txErr := common.ConvertToDecimal(poolDoc, "usage")
		if txErr != nil {
			return txErr
		}
		amount := common.StringToDecimal(order.Amount)
		usage = usage.Sub(amount)
		if usage.LessThan(common.Zero) {
			usage = common.Zero
		}

		userLimitDoc, txErr := tx.Get(docUserLimitRef)
		if txErr != nil {
			return txErr
		}
		userUsage, txErr := common.ConvertToDecimal(userLimitDoc, "usage")
		if txErr != nil {
			return txErr
		}
		fiatAmount := common.StringToDecimal(order.FiatLocalAmount)
		userUsage = userUsage.Sub(fiatAmount)
		if userUsage.LessThan(common.Zero) {
			userUsage = common.Zero
		}

		txErr = tx.Set(docRef, order.GetUpdate(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}
		txErr = tx.Set(docUserRef, order.GetUpdate(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}

		txErr = tx.Delete(docOrderRefRef)
		if txErr != nil {
			return txErr
		}

		txErr = tx.Set(docPoolRef, bean.CoinPool{
			Usage: usage.String(),
		}.GetUpdate(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}

		txErr = tx.Set(docUserLimitRef, bean.CoinUserLimit{
			Usage: userUsage.String(),
		}.GetUpdate(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}

		return txErr
	})

	return err
}

func (dao CoinDao) UpdateCoinOrderReceipt(order *bean.CoinOrder) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinOrderItemPath(order.Id))
	docUserRef := dbClient.Doc(GetCoinOrderUserItemPath(order.UID, order.Id))
	docOrderRefRef := dbClient.Doc(GetCoinOrderRefCodeItemPath(order.RefCode))

	batch := dbClient.Batch()
	batch.Set(docRef, order.GetReceiptUpdate(), firestore.MergeAll)
	batch.Set(docUserRef, order.GetReceiptUpdate(), firestore.MergeAll)
	batch.Delete(docOrderRefRef)
	_, err := batch.Commit(context.Background())

	return err
}

func (dao CoinDao) UpdateCoinOrder(order *bean.CoinOrder) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinOrderItemPath(order.Id))
	docUserRef := dbClient.Doc(GetCoinOrderUserItemPath(order.UID, order.Id))

	batch := dbClient.Batch()
	batch.Set(docRef, order.GetUpdate(), firestore.MergeAll)
	batch.Set(docUserRef, order.GetUpdate(), firestore.MergeAll)
	_, err := batch.Commit(context.Background())

	return err
}

func (dao CoinDao) UpdateCoinSellingOrder(order *bean.CoinSellingOrder) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinSellingOrderItemPath(order.Id))
	docUserRef := dbClient.Doc(GetCoinSellingOrderUserItemPath(order.UID, order.Id))

	batch := dbClient.Batch()
	batch.Set(docRef, order.GetUpdate(), firestore.MergeAll)
	batch.Set(docUserRef, order.GetUpdate(), firestore.MergeAll)
	_, err := batch.Commit(context.Background())

	return err
}

func (dao CoinDao) UpdateCoinOrderReview(order *bean.CoinOrder) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinOrderItemPath(order.Id))
	docUserRef := dbClient.Doc(GetCoinOrderUserItemPath(order.UID, order.Id))

	batch := dbClient.Batch()
	batch.Set(docRef, order.GetReviewUpdate(), firestore.MergeAll)
	batch.Set(docUserRef, order.GetReviewUpdate(), firestore.MergeAll)
	_, err := batch.Commit(context.Background())

	return err
}

func (dao CoinDao) UpdateCoinSellingOrderReview(order *bean.CoinSellingOrder) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinSellingOrderItemPath(order.Id))
	docUserRef := dbClient.Doc(GetCoinSellingOrderUserItemPath(order.UID, order.Id))

	batch := dbClient.Batch()
	batch.Set(docRef, order.GetReviewUpdate(), firestore.MergeAll)
	batch.Set(docUserRef, order.GetReviewUpdate(), firestore.MergeAll)
	_, err := batch.Commit(context.Background())

	return err
}

func (dao CoinDao) FinishCoinOrder(order *bean.CoinOrder) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinOrderItemPath(order.Id))
	docUserRef := dbClient.Doc(GetCoinOrderUserItemPath(order.UID, order.Id))

	batch := dbClient.Batch()
	batch.Set(docRef, order.GetUpdate(), firestore.MergeAll)
	batch.Set(docUserRef, order.GetUpdate(), firestore.MergeAll)
	_, err := batch.Commit(context.Background())

	return err
}

func (dao CoinDao) FinishCoinSellingOrder(order *bean.CoinSellingOrder) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinSellingOrderItemPath(order.Id))
	docUserRef := dbClient.Doc(GetCoinSellingOrderUserItemPath(order.UID, order.Id))

	batch := dbClient.Batch()
	batch.Set(docRef, order.GetUpdate(), firestore.MergeAll)
	batch.Set(docUserRef, order.GetUpdate(), firestore.MergeAll)
	_, err := batch.Commit(context.Background())

	return err
}

func (dao CoinDao) AddCoinBank(coinBank *bean.CoinBank) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinBankCountryItemPath(coinBank.Country, coinBank.Bank))
	_, err := docRef.Set(context.Background(), coinBank.GetAdd())

	return err
}

func (dao CoinDao) UpdateNotificationCoinOrder(order bean.CoinOrder) error {
	dbClient := firebase_service.NotificationFirebaseClient

	ref := dbClient.NewRef(GetNotificationCoinOrderPath(order.UID, order.Id))
	err := ref.Set(context.Background(), order.GetNotificationUpdate())

	return err
}

func (dao CoinDao) UpdateNotificationCoinSellingOrder(order bean.CoinSellingOrder) error {
	dbClient := firebase_service.NotificationFirebaseClient

	ref := dbClient.NewRef(GetNotificationCoinSellingOrderPath(order.UID, order.Id))
	err := ref.Set(context.Background(), order.GetNotificationUpdate())

	return err
}

func (dao CoinDao) ListCoinOrderRefCode() (t TransferObject) {
	ListObjects(GetCoinOrderRefCodePath(), &t, nil, snapshotToCoinOrderRefCode)
	return
}

func (dao CoinDao) ListCoinSellingOrderRefCode() (t TransferObject) {
	ListObjects(GetCoinSellingOrderRefCodePath(), &t, nil, snapshotToCoinOrderRefCode)
	return
}

func (dao CoinDao) GetCoinOrderRefCode(refCode string) (t TransferObject) {
	GetObject(GetCoinOrderRefCodeItemPath(refCode), &t, snapshotToCoinOrderRefCode)
	return
}

func (dao CoinDao) GetCoinSellingOrderRefCode(refCode string) (t TransferObject) {
	GetObject(GetCoinSellingOrderRefCodeItemPath(refCode), &t, snapshotToCoinOrderRefCode)
	return
}

func (dao CoinDao) GetCoinPool(currency string) (t TransferObject) {
	GetObject(GetCoinPoolItemPath(currency), &t, snapshotToCoinPool)
	return
}

func (dao CoinDao) GetCoinSellingPool(currency string) (t TransferObject) {
	GetObject(GetCoinSellingPoolItemPath(currency), &t, snapshotToCoinPool)
	return
}

func (dao CoinDao) GetCoinPayment(orderId string) (t TransferObject) {
	GetObject(GetCoinPaymentItemPath(orderId), &t, snapshotToCoinPayment)
	return
}

func (dao CoinDao) GetCoinSellingPayment(orderId string) (t TransferObject) {
	GetObject(GetCoinSellingPaymentItemPath(orderId), &t, snapshotToCoinSellingPayment)
	return
}

func (dao CoinDao) AddCoinPayment(payment *bean.CoinPayment) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinPaymentItemPath(payment.Order))
	_, err := docRef.Set(context.Background(), payment.GetAdd())

	return err
}

func (dao CoinDao) AddCoinSellingPayment(payment *bean.CoinSellingPayment) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinSellingPaymentItemPath(payment.Order))
	_, err := docRef.Set(context.Background(), payment.GetAdd())

	return err
}

func (dao CoinDao) UpdateCoinPayment(payment *bean.CoinPayment, addAmount decimal.Decimal) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinPaymentItemPath(payment.Order))
	err := dbClient.RunTransaction(context.Background(), func(ctx context.Context, tx *firestore.Transaction) error {
		var txErr error

		paymentDoc, txErr := tx.Get(docRef)
		if txErr != nil {
			return txErr
		}
		amount, txErr := common.ConvertToDecimal(paymentDoc, "fiat_amount")
		if txErr != nil {
			return txErr
		}
		amount = amount.Add(addAmount)
		payment.FiatAmount = amount.String()

		txErr = tx.Set(docRef, payment.GetUpdate(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}

		return txErr
	})

	return err
}

func (dao CoinDao) UpdateCoinSellingPayment(payment *bean.CoinSellingPayment, addAmount decimal.Decimal) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinSellingPaymentItemPath(payment.Order))
	err := dbClient.RunTransaction(context.Background(), func(ctx context.Context, tx *firestore.Transaction) error {
		var txErr error

		paymentDoc, txErr := tx.Get(docRef)
		if txErr != nil {
			return txErr
		}
		amount, txErr := common.ConvertToDecimal(paymentDoc, "amount")
		if txErr != nil {
			return txErr
		}
		amount = amount.Add(addAmount)
		payment.Amount = amount.String()

		txErr = tx.Set(docRef, payment.GetUpdate(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}

		return txErr
	})

	return err
}

func (dao CoinDao) GetCoinReviewCount(direction string) (t TransferObject) {
	GetObject(GetCoinReviewCountItemPath(direction), &t, snapshotToCoinReviewCount)
	return
}

func (dao CoinDao) ListReviews(direction string, limit int, startAt interface{}) (t TransferObject) {
	ListPagingObjects(GetCoinReviewPath(direction), &t, limit, startAt, func(collRef *firestore.CollectionRef) firestore.Query {
		query := collRef.OrderBy("created_at", firestore.Desc)
		return query
	}, snapshotToCoinReview)

	return
}

func (dao CoinDao) AddCoinReview(direction string, review *bean.CoinReview, reviewCount *bean.CoinReviewCount) error {
	dbClient := firebase_service.FirestoreClient
	batch := dbClient.Batch()

	docRef := dbClient.Collection(GetCoinReviewPath(direction)).NewDoc()
	countDocRef := dbClient.Doc(GetCoinReviewCountItemPath(direction))
	review.Id = docRef.ID

	batch.Set(docRef, review.GetAdd())
	batch.Set(countDocRef, reviewCount.GetUpdate(), firestore.MergeAll)

	_, err := batch.Commit(context.Background())

	return err
}

func (dao CoinDao) GetCoinUserLimit(id string) (t TransferObject) {
	GetObject(GetCoinUserLimitItemPath(id), &t, snapshotToCoinUserLimit)
	return
}

func (dao CoinDao) GetCoinSellingUserLimit(id string) (t TransferObject) {
	GetObject(GetCoinSellingUserLimitItemPath(id), &t, snapshotToCoinUserLimit)
	return
}

func (dao CoinDao) ListCoinUserLimit() (t TransferObject) {
	ListObjects(GetCoinUserLimitPath(), &t, func(collRef *firestore.CollectionRef) firestore.Query {
		query := collRef.Where("usage", ">", "0")
		return query
	}, snapshotToCoinUserLimit)
	return
}

func (dao CoinDao) ListCoinSellingUserLimit() (t TransferObject) {
	ListObjects(GetCoinSellingUserLimitPath(), &t, func(collRef *firestore.CollectionRef) firestore.Query {
		query := collRef.Where("usage", ">", "0")
		return query
	}, snapshotToCoinUserLimit)
	return
}

func (dao CoinDao) AddCoinUserLimit(userLimit *bean.CoinUserLimit) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinUserLimitItemPath(userLimit.UID))
	_, err := docRef.Set(context.Background(), userLimit.GetAdd())

	return err
}

func (dao CoinDao) AddCoinSellingUserLimit(userLimit *bean.CoinUserLimit) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinSellingUserLimitItemPath(userLimit.UID))
	_, err := docRef.Set(context.Background(), userLimit.GetAdd())

	return err
}

func (dao CoinDao) UpdateCoinUserLimitLevel(userLimit *bean.CoinUserLimit) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinUserLimitItemPath(userLimit.UID))
	_, err := docRef.Set(context.Background(), userLimit.GetUpdateLevel(), firestore.MergeAll)

	return err
}

func (dao CoinDao) UpdateCoinSellingUserLimitLevel(userLimit *bean.CoinUserLimit) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinSellingUserLimitItemPath(userLimit.UID))
	_, err := docRef.Set(context.Background(), userLimit.GetUpdateLevel(), firestore.MergeAll)

	return err
}

func (dao CoinDao) ResetCoinUserLimit(uid string) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinUserLimitItemPath(uid))
	_, err := docRef.Set(context.Background(), bean.CoinUserLimit{
		Usage: common.Zero.String(),
	}.GetUpdate(), firestore.MergeAll)
	return err
}

func (dao CoinDao) ResetCoinSellingUserLimit(uid string) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinSellingUserLimitItemPath(uid))
	_, err := docRef.Set(context.Background(), bean.CoinUserLimit{
		Usage: common.Zero.String(),
	}.GetUpdate(), firestore.MergeAll)
	return err
}

func (dao CoinDao) UpdateCoinUserLimit(uid string, amount decimal.Decimal, userLimit bean.CoinUserLimit) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinUserLimitItemPath(uid))
	err := dbClient.RunTransaction(context.Background(), func(ctx context.Context, tx *firestore.Transaction) error {
		var txErr error

		paymentDoc, txErr := tx.Get(docRef)
		if txErr != nil {
			return txErr
		}
		usage, txErr := common.ConvertToDecimal(paymentDoc, "usage")
		if txErr != nil {
			return txErr
		}
		usage = usage.Add(amount)
		userLimit.Usage = usage.String()

		txErr = tx.Set(docRef, userLimit.GetUpdate(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}

		return txErr
	})

	return err
}

func (dao CoinDao) UpdateCoinSellingUserLimit(uid string, amount decimal.Decimal, userLimit bean.CoinUserLimit) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinSellingUserLimitItemPath(uid))
	err := dbClient.RunTransaction(context.Background(), func(ctx context.Context, tx *firestore.Transaction) error {
		var txErr error

		paymentDoc, txErr := tx.Get(docRef)
		if txErr != nil {
			return txErr
		}
		usage, txErr := common.ConvertToDecimal(paymentDoc, "usage")
		if txErr != nil {
			return txErr
		}
		usage = usage.Add(amount)
		userLimit.Usage = usage.String()

		txErr = tx.Set(docRef, userLimit.GetUpdate(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}

		return txErr
	})

	return err
}

func (dao CoinDao) GetCoinGenerateAddress(currency string, address string) (t TransferObject) {
	GetObject(GetCoinGenerateAddressItemPath(currency, address), &t, snapshotToCoinGeneratedAddress)
	return
}

func (dao CoinDao) AddCoinGenerateAddress(address *bean.CoinGeneratedAddress) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinGenerateAddressItemPath(address.Currency, address.Address))
	_, err := docRef.Set(context.Background(), address.GetAdd())

	return err
}

func (dao CoinDao) ListCoinAddressTracking(country string) (t TransferObject) {
	ListObjects(GetCoinAddressTrackingPath(country), &t, nil, snapshotToCoinAddressTracking)
	return
}

func (dao CoinDao) GetCoinAddressTracking(currency string, address string) (t TransferObject) {
	GetObject(GetCoinAddressTrackingItemPath(currency, address), &t, snapshotToCoinAddressTracking)
	return
}

func (dao CoinDao) AddCoinAddressTracking(tracking *bean.CoinAddressTracking) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinAddressTrackingItemPath(tracking.Currency, tracking.Address))
	_, err := docRef.Set(context.Background(), tracking.GetAdd())

	return err
}

func (dao CoinDao) RemoveCoinAddressTracking(currency string, address string) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCoinAddressTrackingItemPath(currency, address))
	_, err := docRef.Delete(context.Background())

	return err
}

func GetCoinCenterCountryPath(country string) string {
	return fmt.Sprintf("coin_centers/%s/currency", country)
}

func GetCoinBankCountryPath(country string) string {
	return fmt.Sprintf("coin_banks/%s/items", country)
}

func GetCoinBankCountryItemPath(country string, name string) string {
	return fmt.Sprintf("coin_banks/%s/items/%s", country, name)
}

func GetCoinOrderPath() string {
	return fmt.Sprintf("coin_orders")
}

func GetCoinOrderItemPath(id string) string {
	return fmt.Sprintf("coin_orders/%s", id)
}

func GetCoinSellingOrderPath() string {
	return fmt.Sprintf("coin_selling_orders")
}

func GetCoinSellingOrderItemPath(id string) string {
	return fmt.Sprintf("coin_selling_orders/%s", id)
}

func GetCoinOrderUserItemPath(userId string, id string) string {
	return fmt.Sprintf("coin/%s/orders/%s", userId, id)
}

func GetCoinSellingOrderUserItemPath(userId string, id string) string {
	return fmt.Sprintf("coin_selling/%s/orders/%s", userId, id)
}

func GetNotificationCoinOrderPath(userId string, id string) string {
	return fmt.Sprintf("users/%s/coin/coin_order_%s", userId, id)
}

func GetNotificationCoinSellingOrderPath(userId string, id string) string {
	return fmt.Sprintf("users/%s/coin/coin_selling_order_%s", userId, id)
}

func GetCoinOrderRefCodePath() string {
	return fmt.Sprintf("coin_order_refs")
}

func GetCoinSellingOrderRefCodePath() string {
	return fmt.Sprintf("coin_selling_order_refs")
}

func GetCoinOrderRefCodeItemPath(refCode string) string {
	return fmt.Sprintf("coin_order_refs/%s", refCode)
}

func GetCoinSellingOrderRefCodeItemPath(refCode string) string {
	return fmt.Sprintf("coin_selling_order_refs/%s", refCode)
}

func GetCoinPoolItemPath(currency string) string {
	return fmt.Sprintf("coin_pools/%s", currency)
}

func GetCoinSellingPoolItemPath(currency string) string {
	return fmt.Sprintf("coin_selling_pools/%s", currency)
}

func GetCoinPaymentItemPath(orderId string) string {
	return fmt.Sprintf("coin_payments/%s", orderId)
}

func GetCoinSellingPaymentItemPath(orderId string) string {
	return fmt.Sprintf("coin_selling_payments/%s", orderId)
}

func GetCoinReviewCountItemPath(direction string) string {
	return fmt.Sprintf("coin_reviews/%s", direction)
}

func GetCoinReviewPath(direction string) string {
	return fmt.Sprintf("coin_reviews/%s/items", direction)
}

func GetCoinReviewItemPath(direction string, id string) string {
	return fmt.Sprintf("coin_reviews/%s/items/%s", direction, id)
}

func GetCoinGenerateAddressItemPath(currency string, address string) string {
	return fmt.Sprintf("coin_addresses/%s/items/%s", currency, address)
}

func GetCoinAddressTrackingPath(currency string) string {
	return fmt.Sprintf("coin_address_tracking/%s/items", currency)
}

func GetCoinAddressTrackingItemPath(currency string, address string) string {
	return fmt.Sprintf("coin_address_tracking/%s/items/%s", currency, address)
}

func GetCoinUserLimitPath() string {
	return fmt.Sprintf("coin_user_limits")
}

func GetCoinUserLimitItemPath(id string) string {
	return fmt.Sprintf("coin_user_limits/%s", id)
}

func GetCoinSellingUserLimitPath() string {
	return fmt.Sprintf("coin_selling_user_limits")
}

func GetCoinSellingUserLimitItemPath(id string) string {
	return fmt.Sprintf("coin_selling_user_limits/%s", id)
}

func snapshotToCoinOrder(snapshot *firestore.DocumentSnapshot) interface{} {
	var obj bean.CoinOrder
	snapshot.DataTo(&obj)
	return obj
}

func snapshotToCoinSellingOrder(snapshot *firestore.DocumentSnapshot) interface{} {
	var obj bean.CoinSellingOrder
	snapshot.DataTo(&obj)
	return obj
}

func snapshotToCoinCenter(snapshot *firestore.DocumentSnapshot) interface{} {
	var obj bean.CoinCenter
	snapshot.DataTo(&obj)
	return obj
}

func snapshotToCoinBank(snapshot *firestore.DocumentSnapshot) interface{} {
	var obj bean.CoinBank
	snapshot.DataTo(&obj)
	return obj
}

func snapshotToCoinPayment(snapshot *firestore.DocumentSnapshot) interface{} {
	var obj bean.CoinPayment
	snapshot.DataTo(&obj)
	return obj
}

func snapshotToCoinSellingPayment(snapshot *firestore.DocumentSnapshot) interface{} {
	var obj bean.CoinSellingPayment
	snapshot.DataTo(&obj)
	return obj
}

func snapshotToCoinOrderRefCode(snapshot *firestore.DocumentSnapshot) interface{} {
	var obj bean.CoinOrderRefCode
	snapshot.DataTo(&obj)
	return obj
}

func snapshotToCoinPool(snapshot *firestore.DocumentSnapshot) interface{} {
	var obj bean.CoinPool
	snapshot.DataTo(&obj)
	return obj
}

func snapshotToCoinReview(snapshot *firestore.DocumentSnapshot) interface{} {
	var obj bean.CoinReview
	snapshot.DataTo(&obj)
	return obj
}

func snapshotToCoinReviewCount(snapshot *firestore.DocumentSnapshot) interface{} {
	var obj bean.CoinReviewCount
	snapshot.DataTo(&obj)
	return obj
}

func snapshotToCoinUserLimit(snapshot *firestore.DocumentSnapshot) interface{} {
	var obj bean.CoinUserLimit
	snapshot.DataTo(&obj)
	return obj
}

func snapshotToCoinGeneratedAddress(snapshot *firestore.DocumentSnapshot) interface{} {
	var obj bean.CoinGeneratedAddress
	snapshot.DataTo(&obj)
	return obj
}

func snapshotToCoinAddressTracking(snapshot *firestore.DocumentSnapshot) interface{} {
	var obj bean.CoinAddressTracking
	snapshot.DataTo(&obj)
	return obj
}
