package main

import (
	"context"
	"flag"
	"net/http"
	"os"

	"github.com/go-kit/kit/endpoint"
	kitprometheus "github.com/go-kit/kit/metrics/prometheus"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/go-kit/log"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	var (
		listen = flag.String("listen", ":8080", "HTTP listen address")
		proxy  = flag.String("proxy", "", "Optional comma-separated list of URLs to proxy uppercase requests")
	)
	flag.Parse()

	var logger log.Logger
	logger = log.NewLogfmtLogger(os.Stderr)
	logger = log.With(logger, "listen", *listen, "caller", log.DefaultCaller)

	fieldKeys := []string{"method", "error"}
	requestCount := kitprometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Namespace: "my_group",
		Subsystem: "string_service",
		Name:      "request_count",
		Help:      "Number of requests received.",
	}, fieldKeys)
	requestLatency := kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
		Namespace: "my_group",
		Subsystem: "string_service",
		Name:      "request_latency_microseconds",
		Help:      "Total duration of requests in microseconds.",
	}, fieldKeys)
	countResult := kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
		Namespace: "my_group",
		Subsystem: "string_service",
		Name:      "count_result",
		Help:      "The result of each count method.",
	}, []string{})

	// Use service middlewares for business-domain concerns, like logging and instrumentation.
	var svc StringService
	svc = stringService{}
	// Middleware that acts as a proxy.
	// A proxy is something that stands in between the client and the actual service and
	// forwards requests to another service.
	// In this case, the proxying middleware is supposed to forward requests for the Uppercase method
	// to a different string service.
	// Let's say your original string service is Service A,
	// and you want to proxy the Uppercase method to Service B.
	// The proxying middleware intercepts requests for Uppercase,
	// sends them to Service B, gets the result, and returns it to the caller of Service A.
	// This way, from the perspective of the client, it seems like it's still interacting
	// with Service A, but the Uppercase operation is actually being handled by Service B.
	svc = proxyingMiddleware(context.Background(), *proxy, logger)(svc)
	svc = loggingMiddleware(logger)(svc)
	svc = instrumentingMiddleware(requestCount, requestLatency, countResult)(svc)

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
	http.Handle("/metrics", promhttp.Handler())
	logger.Log("msg", "HTTP", "addr", *listen)
	logger.Log("err", http.ListenAndServe(*listen, nil))
}

// ./stringsrv -listen=:8001
// listen=:8001 caller=proxying.go:26 proxy_to=none
// listen=:8001 caller=main.go:79 msg=HTTP addr=:8001
// listen=:8001 caller=transportMiddleware.go:13 method=uppercase msg="calling endpoint"
// listen=:8001 caller=logging.go:22 method=uppercase input=foo output=FOO err=null took=813ns
// listen=:8001 caller=transportMiddleware.go:15 method=uppercase msg="called endpoint"

// ./stringsrv -listen=:8002
// listen=:8002 caller=proxying.go:26 proxy_to=none
// listen=:8002 caller=main.go:79 msg=HTTP addr=:8002
// listen=:8002 caller=transportMiddleware.go:13 method=uppercase msg="calling endpoint"
// listen=:8002 caller=logging.go:22 method=uppercase input=bar output=BAR err=null took=687ns
// listen=:8002 caller=transportMiddleware.go:15 method=uppercase msg="called endpoint"

// ./stringsrv -listen=:8003
// listen=:8003 caller=proxying.go:26 proxy_to=none
// listen=:8003 caller=main.go:79 msg=HTTP addr=:8003
// listen=:8003 caller=transportMiddleware.go:13 method=uppercase msg="calling endpoint"
// listen=:8003 caller=logging.go:22 method=uppercase input=baz output=BAZ err=null took=768ns
// listen=:8003 caller=transportMiddleware.go:15 method=uppercase msg="called endpoint"

// ./stringsrv -listen=:8080 -proxy=localhost:8001,localhost:8002,localhost:8003
// listen=:8080 caller=proxying.go:45 proxy_to="[localhost:8001 localhost:8002 localhost:8003]"
// listen=:8080 caller=main.go:79 msg=HTTP addr=:8080
// listen=:8080 caller=transportMiddleware.go:13 method=uppercase msg="calling endpoint"
// listen=:8080 caller=logging.go:22 method=uppercase input=foo output=FOO err=null took=3.007225ms
// listen=:8080 caller=transportMiddleware.go:15 method=uppercase msg="called endpoint"
// listen=:8080 caller=transportMiddleware.go:13 method=uppercase msg="calling endpoint"
// listen=:8080 caller=logging.go:22 method=uppercase input=bar output=BAR err=null took=1.160132ms
// listen=:8080 caller=transportMiddleware.go:15 method=uppercase msg="called endpoint"
// listen=:8080 caller=transportMiddleware.go:13 method=uppercase msg="calling endpoint"
// listen=:8080 caller=logging.go:22 method=uppercase input=baz output=BAZ err=null took=1.174695ms
// listen=:8080 caller=transportMiddleware.go:15 method=uppercase msg="called endpoint"

// for s in foo bar baz ; do curl -d"{\"s\":\"$s\"}" localhost:8080/uppercase ; done
