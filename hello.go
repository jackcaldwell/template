package template

import "context"

// HelloService is a basic service used to demonstrate the service pattern
type HelloService interface {
	SayHello(ctx context.Context, request SayHelloRequest) (string, error)
}

// HelloMiddleware describes a service (as opposed to endpoint) middleware for the HelloService.
type HelloMiddleware func(service HelloService) HelloService

// SayHelloRequest represents a request passed to HelloService.SayHello
type SayHelloRequest struct {
	Name string `json:"name"`
}

// SayHelloResponse represents a response returned from HelloService.SayHello
type SayHelloResponse struct {
	Greeting string `json:"greeting"`
}