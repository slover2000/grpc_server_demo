package trace_test

import (
	"log"
	"net/http"

	"github.com/slover2000/prisma/trace"
)

var traceClient *trace.Client

func ExampleHTTPClient_Do() {
	client := http.Client{
		Transport: &trace.Transport{},
	}
	span := traceClient.NewSpan("/foo") // traceClient is a *trace.Client

	req, _ := http.NewRequest("GET", "https://metadata/users", nil)
	req = req.WithContext(trace.NewContext(req.Context(), span))

	if _, err := client.Do(req); err != nil {
		log.Fatal(err)
	}
}

func ExampleClient_HTTPHandler() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client := http.Client{
			Transport: &trace.Transport{},
		}

		req, _ := http.NewRequest("GET", "https://metadata/users", nil)
		req = req.WithContext(r.Context())

		// The outgoing request will be traced with r's trace ID.
		if _, err := client.Do(req); err != nil {
			log.Fatal(err)
		}
	})
	http.Handle("/foo", traceClient.HTTPHandler(handler)) // traceClient is a *trace.Client
}