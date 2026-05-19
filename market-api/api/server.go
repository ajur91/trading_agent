package api

import (
	"context"
	"net/http"
	"time"

	"github.com/beto/trading-agent/market-api/internal/okx"
	"github.com/sirupsen/logrus"
)

type Server struct {
	okx      *okx.Client
	log      *logrus.Logger
	leverage int
	mux      *http.ServeMux
}

func NewServer(client *okx.Client, log *logrus.Logger, leverage int) *Server {
	s := &Server{
		okx:      client,
		log:      log,
		leverage: leverage,
		mux:      http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/v1/market/candles", s.handleCandles)
	s.mux.HandleFunc("/v1/market/indicators", s.handleIndicators)
	s.mux.HandleFunc("/v1/account/balance", s.handleBalance)
	s.mux.HandleFunc("/v1/account/positions", s.handlePositions)
	s.mux.HandleFunc("/v1/orders/place", s.handlePlaceOrder)
	s.mux.HandleFunc("/v1/orders/close", s.handleCloseOrder)
}

func (s *Server) Start(addr string) error {
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}
	s.log.Infof("market-api listening on %s", addr)
	return srv.ListenAndServe()
}

func (s *Server) requestCtx(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 25*time.Second)
}
