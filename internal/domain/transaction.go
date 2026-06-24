package domain

type TransactionType string

const (
	TxTypeDeposit  TransactionType = "deposit"
	TxTypeWithdraw TransactionType = "withdraw"
)
