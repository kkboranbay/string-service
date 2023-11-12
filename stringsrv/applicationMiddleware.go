package main

import (
	"time"

	"github.com/go-kit/log"
)

type applicationLoggingMiddleware struct {
	logger log.Logger
	next   StringService
}

func (mw applicationLoggingMiddleware) Uppercase(s string) (output string, err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "uppercase",
			"input", s,
			"output", output,
			"err", err,
			"took", time.Since(begin),
		)
	}(time.Now())

	output, err = mw.next.Uppercase(s)
	return
}

func (mw applicationLoggingMiddleware) Count(s string) (n int) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "count",
			"input", s,
			"output", n,
			"took", time.Since(begin),
		)

	}(time.Now())

	n = len(s)
	return
}
