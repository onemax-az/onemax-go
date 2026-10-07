package onemax

import "time"

type Reason string

const (
	ReasonOK               Reason = "ok"
	ReasonNotSubscribed    Reason = "not_subscribed"
	ReasonLimitReached     Reason = "limit_reached"
	ReasonOfferUnavailable Reason = "offer_unavailable"
	ReasonInvalidCode      Reason = "invalid_code"
	ReasonNotLinked        Reason = "not_linked"
)

type UsageStatus string

const (
	UsageStatusPending   UsageStatus = "pending"
	UsageStatusConfirmed UsageStatus = "confirmed"
	UsageStatusVoided    UsageStatus = "voided"
)

type Offer struct {
	ID       string `json:"id"`
	Headline string `json:"headline"`
	Title    string `json:"title"`
	OpenNow  bool   `json:"open_now"`
}

type Context struct {
	PartnerName string  `json:"partner_name"`
	VenueID     string  `json:"venue_id"`
	VenueName   string  `json:"venue_name"`
	DailyLimit  int     `json:"daily_limit"`
	Offers      []Offer `json:"offers"`
}

type Verification struct {
	Active     bool    `json:"active"`
	Usable     bool    `json:"usable"`
	Reason     Reason  `json:"reason"`
	UsageID    *string `json:"usage_id"`
	DailyLimit int     `json:"daily_limit"`
}

type Usage struct {
	UsageID     string      `json:"usage_id"`
	Status      UsageStatus `json:"status"`
	Reference   *string     `json:"reference"`
	CreatedAt   time.Time   `json:"created_at"`
	ConfirmedAt *time.Time  `json:"confirmed_at"`
	VoidedAt    *time.Time  `json:"voided_at"`
}

type LinkToken struct {
	LinkToken string `json:"link_token"`
	Active    bool   `json:"active"`
}

type LinkStatus struct {
	Linked bool `json:"linked"`
	Active bool `json:"active"`
}
