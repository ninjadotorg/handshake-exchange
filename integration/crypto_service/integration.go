package crypto_service

import (
	"github.com/autonomousdotai/handshake-exchange/bean"
	"github.com/autonomousdotai/handshake-exchange/common"
	"github.com/autonomousdotai/handshake-exchange/integration/blockchainio_service"
	"github.com/autonomousdotai/handshake-exchange/integration/ethereum_service"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
)

func GetBalance(currency string) (decimal.Decimal, error) {
	if currency == bean.ETH.Code {
		client := ethereum_service.EthereumClient{}
		return client.GetBalance()
	} else if currency == bean.BTC.Code {
		client := blockchainio_service.BlockChainIOClient{}
		return client.GetBalance()
	}

	return common.Zero, errors.New("Currency not support")
}

func SendTransaction(address string, amountStr string, currency string) (string, error) {
	amount, _ := decimal.NewFromString(amountStr)
	if currency == bean.ETH.Code {
		client := ethereum_service.EthereumClient{}
		return client.SendTransaction(address, amount)
	} else if currency == bean.BTC.Code {
		client := blockchainio_service.BlockChainIOClient{}
		return client.SendTransaction(address, amount)
	}

	return "", errors.New("Currency not support")
}