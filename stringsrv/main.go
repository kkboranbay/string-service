package main

import (
	"net/http"
	"os"

	"github.com/go-kit/kit/endpoint"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/go-kit/log"
)

func main() {
	logger := log.NewLogfmtLogger(os.Stderr)

	// Use service middlewares for business-domain concerns, like logging and instrumentation.
	var svc StringService
	svc = stringService{}
	svc = applicationLoggingMiddleware{logger, svc}

	// Use endpoint middlewares for transport-domain concerns, like circuit breaking and rate limiting.
	var uppercase endpoint.Endpoint
	uppercase = makeUppercaseEndpoint(svc)
	uppercase = transportLoggingMiddleware(log.With(logger, "method", "uppercase"))(uppercase)

	var count endpoint.Endpoint
	count = makeCountEndpoint(svc)
	count = transportLoggingMiddleware(log.With(logger, "method", "count"))(count)

	uppercaseHandler := httptransport.NewServer(
		uppercase,
		decodeUppercaseRequest,
		encodeResponse,
	)

	countHandler := httptransport.NewServer(
		count,
		decodeCountRequest,
		encodeResponse,
	)

	http.Handle("/uppercase", uppercaseHandler)
	http.Handle("/count", countHandler)
	http.ListenAndServe(":8080", nil)
}
