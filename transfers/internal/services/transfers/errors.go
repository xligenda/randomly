package transfers

import "errors"

var (
	ErrAmountLimitExceeded       = errors.New("amount exceeds maximum supported by payment system")
	ErrNegativeAmount            = errors.New("amount is negative or equals to zero")
	ErrInvalidSignature          = errors.New("invalid webhook signature")
	ErrFailedToCreatePayment     = errors.New("failed to create payment")
	ErrFailedToPersist           = errors.New("failed to persist transfer")
	ErrNotFound                  = errors.New("transfer not found")
	ErrNothingToDo               = errors.New("nothing to do")
	ErrFailedToParse             = errors.New("failed to parse payment payload")
	ErrFailedToConfirmPayment    = errors.New("failed to confirm payment")
	ErrNoEligibleReceiver        = errors.New("no eligible receiver found after max attempts")
	ErrFailedToLeaseReceiver     = errors.New("failed to lease receiver")
	ErrReceiverAlreadyLeased     = errors.New("receiver already leased by another worker")
	ErrUserCardsInvalid          = errors.New("receiver has no valid cards")
	ErrSelectionCardMismatch     = errors.New("card does not belong to the selected receiver")
	ErrSelectionReceiverMismatch = errors.New("receiver uuid does not match the supplied username")
	ErrFailedToFetchMCProfile    = errors.New("failed to fetch minecraft player profile")
	ErrFailedToCreateTransaction = errors.New("failed to create spworlds transaction")
)
