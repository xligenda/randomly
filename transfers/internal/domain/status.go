package domain

type Status string

const (
	StatusCreated          Status = "created"       // record created, payment url generated
	StatusPaid             Status = "paid"          // webhook comfirmed payment receiver
	StatusSelectedReceiver Status = "user_selected" // worker already selected Receiver, so now selecting card
	StatusNotSelected      Status = "not_selected"  // issuer did not get in time, worker can start auto selecting
	StatusSent             Status = "sent"          // Sender selecred Receiver, server transfered Amount

	StatusFailed Status = "failed" // worker received error
)

func (s Status) String() string {
	return string(s)
}
