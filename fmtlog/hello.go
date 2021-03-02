package fmtlog

import (
	"context"
	"github.com/go-kit/kit/log"
	"template"
	"time"
)

func HelloLoggingMiddleware(logger log.Logger) template.HelloMiddleware {
	return func (next template.HelloService) template.HelloService {
		return &helloLoggingMiddleware{
			next,
			logger,
		}
	}
}

type helloLoggingMiddleware struct {
	next template.HelloService
	logger log.Logger
}

func (mw *helloLoggingMiddleware) SayHello(ctx context.Context, request template.SayHelloRequest) (resp string, err error) {
	defer func(begin time.Time) {
		mw.logger.Log("method", "SayHello", "name", request.Name, "took", time.Since(begin), "err", err)
	}(time.Now())

	return mw.next.SayHello(ctx, request)
}


