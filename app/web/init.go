package web

import (
	"context"
	"net"
	"net/http"

	"github.com/wzshiming/anyproxy"
	"github.com/wzshiming/cmux/pattern"
)

func init() {
	anyproxy.Register("view", NewServeConn)
}

func NewServeConn(ctx context.Context, scheme string, address string, conf *anyproxy.Config) (anyproxy.ServeConn, []string, error) {
	var patterns []string

	tmp := pattern.Pattern[pattern.HTTP]
	patterns = make([]string, 0, len(tmp)+1)
	for _, t := range tmp {
		patterns = append(patterns, t+"/")
	}
	s := http.Server{
		BaseContext: func(listener net.Listener) context.Context {
			return ctx
		},
		Handler: Handler(),
	}
	return anyproxy.NewHttpServeConn(&s), patterns, nil
}
