package domain

type Status string

const (
	StatusCreated       Status = "created"   // record created, payment url generated
	StatusPaid          Status = "paid"      // webhook comfirmed payment receiver
	StatusSelectingCard Status = "selecting" // worker already selected Receiver, so now selecting card
	StatusSent          Status = "sent"      // Sender selecred Receiver, server transfered Amount

	StatusFailed Status = "failed" // worker received error
)

func (s Status) String() string {
	return string(s)
}
