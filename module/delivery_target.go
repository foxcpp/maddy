package module

type DeliveryTarget interface {
	Start(ctx *DeliveryContext, mailFrom string) (Delivery, error)
}

type Delivery interface {
	AddRcpt(rcptTo string) error
	Body(body BodyBuffer) error

	Abort() error
	Commit() error
}
