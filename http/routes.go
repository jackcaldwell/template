package http

import (
	"context"
	"github.com/go-kit/kit/endpoint"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"net/http"
	"template"
)

//func (s *Server) registerRoutes() {
//	s.router.Handle("/api/hello/{name}", s.authenticate(s.querySnippetsHandler())).Methods("GET")
//}

type HelloEndpoints struct {
	SayHelloEndpoint endpoint.Endpoint
}

// MakeServerEndpoints returns an Endpoints struct where each endpoint invokes
// the corresponding method on the provided service. Useful in a server.
func MakeServerEndpoints(s template.HelloService) HelloEndpoints {
	return HelloEndpoints{
		SayHelloEndpoint: MakeSayHelloEndpoint(s),
	}
}

// MakePostProfileEndpoint returns an endpoint via the passed service.
// Primarily useful in a server.
func MakeSayHelloEndpoint(s template.HelloService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(template.SayHelloRequest)
		response, err = s.SayHello(ctx, req)
		return
	}
}

// registerRoutes sets up handlers for all of the service endpoints.
func (s *Server) registerRoutes() {
	e := MakeServerEndpoints(s.HelloService)

	s.router.Handle("/api/hello/{name}",
		httptransport.NewServer(
		e.SayHelloEndpoint,
		decodeSayHelloRequest,
		encodeResponse,
	)).Methods("GET")
}

func decodeSayHelloRequest(_ context.Context, r *http.Request) (request interface{}, err error) {
	vars := mux.Vars(r)
	name, ok := vars["name"]
	if !ok {
		return nil, template.Errorf("Invalid value for param 'name'.", template.EINVALID)
	}
	return template.SayHelloRequest{Name: name}, nil
}
