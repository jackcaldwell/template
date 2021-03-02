package pg

import (
	"context"
	"fmt"
	"template"
)

// HelloService represents an example service
type HelloService struct {
	db *DB
}

// NewHello service returns a new instance of HelloService attached to DB.
func NewHelloService(db *DB) template.HelloService {
	return &HelloService{db: db}
}

func (h HelloService) SayHello(ctx context.Context, request template.SayHelloRequest) (string, error) {
	return fmt.Sprintf("Hello, %s", request.Name), nil
}
