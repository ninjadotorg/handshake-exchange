package dao

import (
	"cloud.google.com/go/firestore"
	"context"
	"fmt"
	"github.com/ninjadotorg/handshake-exchange/bean"
	"github.com/ninjadotorg/handshake-exchange/common"
	"github.com/ninjadotorg/handshake-exchange/integration/firebase_service"
)

type CreditDao struct {
}

func (dao CreditDao) GetCredit(userId string) (t TransferObject) {
	GetObject(GetCreditUserPath(userId), &t, snapshotToCredit)
	return
}

func (dao CreditDao) GetCreditItem(userId string, currency string) (t TransferObject) {
	GetObject(GetCreditItemItemPath(userId, currency), &t, snapshotToCreditItem)
	return
}

func (dao CreditDao) ListCreditItem(userId string) (t TransferObject) {
	ListObjects(GetCreditItemPath(userId), &t, nil, snapshotToCreditItem)
	return
}

func (dao CreditDao) AddCredit(credit *bean.Credit) error {
	dbClient := firebase_service.FirestoreClient

	creditPath := GetCreditUserPath(credit.UID)
	docRef := dbClient.Doc(creditPath)
	_, err := docRef.Set(context.Background(), credit.GetAdd())

	return err
}

func (dao CreditDao) UpdateCredit(credit *bean.Credit) error {
	dbClient := firebase_service.FirestoreClient

	creditPath := GetCreditUserPath(credit.UID)
	docRef := dbClient.Doc(creditPath)
	_, err := docRef.Set(context.Background(), credit.GetUpdate(), firestore.MergeAll)

	return err
}

func (dao CreditDao) AddCreditItem(item *bean.CreditItem) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCreditItemItemPath(item.UID, item.Currency))
	_, err := docRef.Set(context.Background(), item.GetAdd())

	return err
}

func (dao CreditDao) UpdateCreditItem(item *bean.CreditItem) error {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCreditItemItemPath(item.UID, item.Currency))
	_, err := docRef.Set(context.Background(), item.GetUpdateStatus(), firestore.MergeAll)

	return err
}

func (dao CreditDao) GetCreditDeposit(currency string, depositId string) (t TransferObject) {
	t = dao.GetCreditDepositByPath(GetCreditDepositItemPath(currency, depositId))
	return
}

func (dao CreditDao) GetCreditDepositByPath(path string) (t TransferObject) {
	GetObject(path, &t, snapshotToCreditDeposit)
	return
}

func (dao CreditDao) AddCreditDeposit(item *bean.CreditItem, deposit *bean.CreditDeposit) (err error) {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Collection(GetCreditDepositPath(deposit.Currency)).NewDoc()
	deposit.Id = docRef.ID
	docUserRef := dbClient.Doc(GetCreditDepositItemUserPath(deposit.UID, deposit.Currency, deposit.Id))

	batch := dbClient.Batch()
	batch.Set(docRef, deposit.GetAdd())
	batch.Set(docUserRef, deposit.GetAdd())
	_, err = batch.Commit(context.Background())

	return err
}

func (dao CreditDao) FinishDepositCreditItem(item *bean.CreditItem, deposit *bean.CreditDeposit,
	itemHistory *bean.CreditBalanceHistory,
	pool *bean.CreditPool, poolOrder *bean.CreditPoolOrder, poolHistory *bean.CreditPoolBalanceHistory,
	tracking *bean.CreditOnChainActionTracking) (err error) {

	dbClient := firebase_service.FirestoreClient
	itemDocRef := dbClient.Doc(GetCreditItemItemPath(deposit.UID, deposit.Currency))
	depositUserDocRef := dbClient.Doc(GetCreditDepositItemUserPath(deposit.UID, deposit.Currency, deposit.Id))
	depositDocRef := dbClient.Doc(GetCreditDepositItemPath(deposit.Currency, deposit.Id))

	poolDocRef := dbClient.Doc(GetCreditPoolItemPath(deposit.Currency, pool.Level))
	poolOrderDocRef := dbClient.Doc(GetCreditPoolItemOrderItemPath(deposit.Currency, pool.Level, poolOrder.Id))
	poolOrderUserDocRef := dbClient.Doc(GetCreditPoolItemOrderItemUserPath(deposit.Currency, deposit.UID, poolOrder.Id))

	balanceHistoryDocRef := dbClient.Collection(GetCreditBalanceHistoryPath(deposit.UID, deposit.Currency)).NewDoc()
	itemHistory.Id = balanceHistoryDocRef.ID
	poolBalanceHistoryDocRef := dbClient.Collection(GetCreditPoolBalanceHistoryPath(deposit.Currency, pool.Level)).NewDoc()
	poolHistory.Id = poolBalanceHistoryDocRef.ID

	docLogRef := dbClient.Doc(GetCreditOnChainActionLogItemPath(tracking.Currency, tracking.Id))
	docTrackingRef := dbClient.Doc(GetCreditOnChainActionTrackingItemPath(tracking.Currency, tracking.Id))

	amount := common.StringToDecimal(deposit.Amount)
	poolOrderUserDocRefs := make([]*firestore.DocumentRef, 0)
	poolOrderDocRefs := make([]*firestore.DocumentRef, 0)

	err = dbClient.RunTransaction(context.Background(), func(ctx context.Context, tx *firestore.Transaction) error {
		var txErr error

		itemDoc, txErr := tx.Get(itemDocRef)
		if txErr != nil {
			return txErr
		}
		itemBalance, txErr := common.ConvertToDecimal(itemDoc, "balance")
		if txErr != nil {
			return txErr
		}
		itemHistory.Old = itemBalance.String()
		itemHistory.Change = amount.String()

		poolDoc, txErr := tx.Get(poolDocRef)
		if err != nil {
			return txErr
		}
		poolBalance, txErr := common.ConvertToDecimal(poolDoc, "balance")
		if txErr != nil {
			return txErr
		}
		poolHistory.Old = poolBalance.String()
		poolHistory.Change = amount.String()

		itemBalance = itemBalance.Add(amount)
		item.Balance = itemBalance.String()
		itemHistory.New = item.Balance

		poolBalance = poolBalance.Add(amount)
		pool.Balance = poolBalance.String()
		poolHistory.New = pool.Balance

		// Update balance
		txErr = tx.Set(itemDocRef, item.GetUpdate(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}
		txErr = tx.Set(poolDocRef, pool.GetUpdate(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}

		// Update status Deposit
		txErr = tx.Set(depositUserDocRef, deposit.GetUpdate(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}
		txErr = tx.Set(depositDocRef, deposit.GetUpdate(), firestore.MergeAll)
		if txErr != nil {
			return txErr
		}

		// Insert pool order
		if amount.GreaterThanOrEqual(common.Zero) {
			txErr = tx.Set(poolOrderDocRef, poolOrder.GetAdd())
			if txErr != nil {
				return txErr
			}
			txErr = tx.Set(poolOrderUserDocRef, poolOrder.GetAdd())
			if txErr != nil {
				return txErr
			}
		} else {
			// Remove all order of this user
			for i, itemDocRef := range poolOrderUserDocRefs {
				txErr = tx.Delete(itemDocRef)
				if txErr != nil {
					return txErr
				}
				txErr = tx.Delete(poolOrderDocRefs[i])
				if txErr != nil {
					return txErr
				}
			}
		}

		// Insert history
		txErr = tx.Set(balanceHistoryDocRef, itemHistory.GetAdd())
		if txErr != nil {
			return txErr
		}
		txErr = tx.Set(poolBalanceHistoryDocRef, poolHistory.GetAdd())
		if txErr != nil {
			return txErr
		}

		// Update tracking
		tx.Delete(docTrackingRef)
		tx.Set(docLogRef, tracking.GetUpdate(), firestore.MergeAll)

		return txErr
	})

	return err
}

func (dao CreditDao) FinishFailedDepositCreditItem(item *bean.CreditItem, deposit *bean.CreditDeposit,
	tracking *bean.CreditOnChainActionTracking) (err error) {

	dbClient := firebase_service.FirestoreClient
	batch := dbClient.Batch()

	itemDocRef := dbClient.Doc(GetCreditItemItemPath(deposit.UID, deposit.Currency))
	depositUserDocRef := dbClient.Doc(GetCreditDepositItemUserPath(deposit.UID, deposit.Currency, deposit.Id))
	depositDocRef := dbClient.Doc(GetCreditDepositItemPath(deposit.Currency, deposit.Id))

	docLogRef := dbClient.Doc(GetCreditOnChainActionLogItemPath(tracking.Currency, tracking.Id))
	docTrackingRef := dbClient.Doc(GetCreditOnChainActionTrackingItemPath(tracking.Currency, tracking.Id))

	batch.Set(itemDocRef, item.GetUpdate(), firestore.MergeAll)
	batch.Set(depositUserDocRef, deposit.GetUpdate(), firestore.MergeAll)
	batch.Set(depositDocRef, deposit.GetUpdate(), firestore.MergeAll)
	batch.Delete(docTrackingRef)
	batch.Set(docLogRef, tracking.GetUpdate(), firestore.MergeAll)
	_, err = batch.Commit(context.Background())

	return err
}

func (dao CreditDao) GetCreditPool(currency string, percentage int) (t TransferObject) {
	level := fmt.Sprintf("%03d", percentage)
	GetObject(GetCreditPoolItemPath(currency, level), &t, snapshotToCreditPool)
	return
}

func (dao CreditDao) AddCreditPool(pool *bean.CreditPool) error {
	dbClient := firebase_service.FirestoreClient

	poolDocRef := dbClient.Doc(GetCreditPoolItemPath(pool.Currency, pool.Level))
	_, err := poolDocRef.Set(context.Background(), pool.GetAdd())

	return err
}

func (dao CreditDao) ListCreditOnChainActionTracking(currency string) (t TransferObject) {
	ListObjects(GetCreditOnChainActionTrackingPath(currency), &t, nil, snapshotToCreditOnChainTracking)
	return
}

func (dao CreditDao) GetCreditOnChainActionTracking(currency string) (t TransferObject) {
	GetObject(GetCreditOnChainActionTrackingPath(currency), &t, snapshotToCreditOnChainTracking)
	return
}

func (dao CreditDao) AddCreditOnChainActionTracking(item *bean.CreditItem, deposit *bean.CreditDeposit,
	tracking *bean.CreditOnChainActionTracking) (err error) {

	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Collection(GetCreditOnChainActionLogPath(tracking.Currency)).NewDoc()
	tracking.Id = docRef.ID
	docTrackingRef := dbClient.Doc(GetCreditOnChainActionTrackingItemPath(tracking.Currency, tracking.Id))

	itemDocRef := dbClient.Doc(GetCreditItemItemPath(deposit.UID, deposit.Currency))
	depositDocRef := dbClient.Doc(GetCreditDepositItemPath(deposit.Currency, deposit.Id))
	depositUserDocRef := dbClient.Doc(GetCreditDepositItemUserPath(deposit.UID, deposit.Currency, deposit.Id))

	batch := dbClient.Batch()
	batch.Set(docRef, tracking.GetAdd())
	batch.Set(docTrackingRef, tracking.GetAdd())
	batch.Set(itemDocRef, item.GetUpdateStatus(), firestore.MergeAll)
	batch.Set(depositDocRef, deposit.GetUpdate(), firestore.MergeAll)
	batch.Set(depositUserDocRef, deposit.GetUpdate(), firestore.MergeAll)
	_, err = batch.Commit(context.Background())

	return err
}

func (dao CreditDao) UpdateCreditOnChainActionTracking(tracking *bean.CreditOnChainActionTracking) (err error) {
	dbClient := firebase_service.FirestoreClient

	docRef := dbClient.Doc(GetCreditOnChainActionLogItemPath(tracking.Currency, tracking.Id))
	docTrackingRef := dbClient.Doc(GetCreditOnChainActionTrackingItemPath(tracking.Currency, tracking.Id))

	batch := dbClient.Batch()
	batch.Delete(docTrackingRef)
	batch.Set(docRef, tracking.GetUpdate(), firestore.MergeAll)
	_, err = batch.Commit(context.Background())

	return err
}

func GetCreditUserPath(userId string) string {
	return fmt.Sprintf("credits/%s", userId)
}

func GetCreditItemPath(userId string) string {
	return fmt.Sprintf("credits/%s/items", userId)
}

func GetCreditItemItemPath(userId string, currency string) string {
	return fmt.Sprintf("credits/%s/items/%s", userId, currency)
}

func GetCreditBalanceHistoryPath(userId string, currency string) string {
	return fmt.Sprintf("credits/%s/items/%s/history", userId, currency)
}

func GetCreditBalanceHistoryItemPath(userId string, currency string, id string) string {
	return fmt.Sprintf("credits/%s/items/%s/history/%s", userId, currency, id)
}

func GetCreditDepositUserPath(userId string, currency string) string {
	return fmt.Sprintf("credits/%s/items/%s/deposits", userId, currency)
}

func GetCreditDepositItemUserPath(userId string, currency string, id string) string {
	return fmt.Sprintf("credits/%s/items/%s/deposits/%s", userId, currency, id)
}

func GetCreditDepositPath(currency string) string {
	return fmt.Sprintf("credit_deposits/%s/deposits", currency)
}

func GetCreditDepositItemPath(currency string, id string) string {
	return fmt.Sprintf("credit_deposits/%s/deposits/%s", currency, id)
}

func GetCreditWithdrawUserPath(userId string) string {
	return fmt.Sprintf("credits/%s/withdraws", userId)
}

func GetCreditWithdrawItemUserPath(userId string, id string) string {
	return fmt.Sprintf("credits/%s/withdraws/%s", userId, id)
}

func GetCreditWithdrawPath() string {
	return fmt.Sprintf("credit_withdraws")
}

func GetCreditWithdrawItemPath(id string) string {
	return fmt.Sprintf("credit_withdraws/%s", id)
}

func GetCreditTransactionUserPath(userId string, currency string) string {
	return fmt.Sprintf("credits/%s/items/%s/transactions", userId, currency)
}

func GetCreditTransactionItemUserPath(userId string, currency string, id string) string {
	return fmt.Sprintf("credits/%s/items/%s/transactions/%s", userId, currency, id)
}

func GetCreditTransactionPath(currency string) string {
	return fmt.Sprintf("credit_transactions/%s/transactions", currency)
}

func GetCreditTransactionItemPath(currency string, id string) string {
	return fmt.Sprintf("credit_transactions/%s/transactions/%s", currency, id)
}

func GetCreditPoolPath(currency string) string {
	return fmt.Sprintf("credit_pools/%s/items", currency)
}

func GetCreditPoolItemPath(currency string, level string) string {
	return fmt.Sprintf("credit_pools/%s/items/%s", currency, level)
}

func GetCreditPoolItemOrderPath(currency string, level string) string {
	return fmt.Sprintf("credit_pools/%s/items/%s/orders", currency, level)
}

func GetCreditPoolItemOrderItemPath(currency string, level string, order string) string {
	return fmt.Sprintf("credit_pools/%s/items/%s/orders/%s", currency, level, order)
}

func GetCreditPoolItemOrderItemUserPath(currency string, userId string, order string) string {
	return fmt.Sprintf("credit_pool_orders/%s/items/%s/orders/%s", currency, userId, order)
}

func GetCreditPoolBalanceHistoryPath(currency string, level string) string {
	return fmt.Sprintf("credit_pools/%s/items/%s/history", currency, level)
}

func GetCreditPoolBalanceHistoryItemPath(currency string, level string, id string) string {
	return fmt.Sprintf("credit_pools/%s/items/%s/history/%s", currency, level, id)
}

func GetCreditOnChainActionTrackingPath(currency string) string {
	return fmt.Sprintf("credit_on_chain_trackings/%s/items", currency)
}

func GetCreditOnChainActionTrackingItemPath(currency string, id string) string {
	return fmt.Sprintf("credit_on_chain_trackings/%s/items/%s", currency, id)
}

func GetCreditOnChainActionLogPath(currency string) string {
	return fmt.Sprintf("credit_on_chain_logs/%s/items", currency)
}

func GetCreditOnChainActionLogItemPath(currency string, id string) string {
	return fmt.Sprintf("credit_on_chain_logs/%s/items/%s", currency, id)
}

func snapshotToCredit(snapshot *firestore.DocumentSnapshot) interface{} {
	var obj bean.Credit
	snapshot.DataTo(&obj)
	return obj
}

func snapshotToCreditItem(snapshot *firestore.DocumentSnapshot) interface{} {
	var obj bean.CreditItem
	snapshot.DataTo(&obj)
	return obj
}

func snapshotToCreditBalanceHistory(snapshot *firestore.DocumentSnapshot) interface{} {
	var obj bean.CreditBalanceHistory
	snapshot.DataTo(&obj)
	obj.Id = snapshot.Ref.ID
	return obj
}

func snapshotToCreditDeposit(snapshot *firestore.DocumentSnapshot) interface{} {
	var obj bean.CreditDeposit
	snapshot.DataTo(&obj)
	obj.Id = snapshot.Ref.ID
	return obj
}

func snapshotToCreditWithdraw(snapshot *firestore.DocumentSnapshot) interface{} {
	var obj bean.CreditWithdraw
	snapshot.DataTo(&obj)
	obj.Id = snapshot.Ref.ID
	return obj
}

func snapshotToCreditPool(snapshot *firestore.DocumentSnapshot) interface{} {
	var obj bean.CreditPool
	snapshot.DataTo(&obj)

	return obj
}

func snapshotToCreditPoolBalanceHistory(snapshot *firestore.DocumentSnapshot) interface{} {
	var obj bean.CreditPoolBalanceHistory
	snapshot.DataTo(&obj)
	obj.Id = snapshot.Ref.ID
	return obj
}

func snapshotToCreditOnChainTracking(snapshot *firestore.DocumentSnapshot) interface{} {
	var obj bean.CreditOnChainActionTracking
	snapshot.DataTo(&obj)
	obj.Id = snapshot.Ref.ID
	return obj
}
