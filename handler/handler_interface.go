package handler

type Func func(req Request) (resp Response)

type Handler interface {
	Run(handlerFunc Func) error
	Start(handlerFunc Func) error
	Stop()

	RegisterAdditionalFunc(handlerFunc Func)

	SendToHealthManager(topic string, payload interface{}) error
}
