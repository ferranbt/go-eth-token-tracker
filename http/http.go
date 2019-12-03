package http

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/ferranbt/go-eth-token-tracker/store"
	"github.com/go-chi/chi"
	"github.com/umbracle/go-web3"
)

// Config is the configuration of the http server
type Config struct {
	Addr string `mapstructure:"addr"`
}

// DefaultConfig returns the default configuration of the http server
func DefaultConfig() *Config {
	return &Config{
		Addr: "127.0.0.1:5000",
	}
}

// Server is the http server
type Server struct {
	logger *log.Logger
	srv    *http.Server
	router chi.Router
	store  store.Store
}

// NewServer creates a new http server
func NewServer(logger *log.Logger, config *Config, store store.Store) (*Server, error) {
	s := &Server{
		logger: logger,
		store:  store,
	}
	s.registerEndpoints()

	s.srv = &http.Server{Addr: config.Addr}
	s.srv.Handler = s.router

	go func() {
		if err := s.srv.ListenAndServe(); err != nil {
			// TODO
		}
	}()

	logger.Printf("[INFO] Http server started at %s", config.Addr)

	return s, nil
}

type endpoint func(r *http.Request) (interface{}, error)

// Stop stops the http server
func (s *Server) Stop() {
	s.srv.Shutdown(context.Background())
}

func (s *Server) registerEndpoints() {
	s.router = chi.NewRouter()

	s.router.Route("/tokens", func(r chi.Router) {
		r.Get("/", s.wrap(s.listTokens))
		r.Get("/{token}", s.wrap(s.listTokenTransfers))
	})
	s.router.Route("/from", func(r chi.Router) {
		r.Get("/{address}", s.wrap(s.listFromTransfers))
	})
	s.router.Route("/to", func(r chi.Router) {
		r.Get("/{address}", s.wrap(s.listToTransfers))
	})
}

type apiResult struct {
	Status string
	Result interface{}
}

func (s *Server) wrap(handler endpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handleErr := func(err error) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		}

		resp, err := handler(r)

		status := "SUCCESS"
		if err != nil {
			status = "ERROR"
		}
		result := apiResult{
			Status: status,
			Result: resp,
		}

		resultJSON, err := json.Marshal(result)
		if err != nil {
			handleErr(err)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(resultJSON)
	}
}

func parseSingleInt(r *http.Request, name string) (int, bool) {
	vals := r.URL.Query()

	raw, ok := vals[name]
	if !ok {
		return 0, false
	}
	if len(raw) != 1 {
		return 0, false
	}

	val, err := strconv.Atoi(raw[0])
	if err != nil {
		return 0, false
	}
	return val, true
}

func parsePagination(r *http.Request, defaultLimit ...int) store.QueryPagination {
	res := store.QueryPagination{}
	var ok bool
	if res.Limit, ok = parseSingleInt(r, "limit"); !ok {
		if len(defaultLimit) == 1 {
			res.Limit = defaultLimit[0]
		}
	}
	res.Offset, _ = parseSingleInt(r, "offset")
	return res
}

func (s *Server) listTokens(r *http.Request) (interface{}, error) {
	query := parsePagination(r, 100)

	tokens, err := s.store.ListTokens(query)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return []string{}, nil
	}
	return tokens, nil
}

func (s *Server) listAccountTransfers(r *http.Request) (interface{}, error) {
	return nil, nil
}

func parseAddresses(r *http.Request, name string) ([]web3.Address, error) {
	vals := r.URL.Query()

	raw, ok := vals[name]
	if !ok {
		return []web3.Address{}, nil
	}

	res := make([]web3.Address, len(raw))
	for indx, i := range raw {
		if err := res[indx].UnmarshalText([]byte(i)); err != nil {
			return nil, err
		}
	}

	return res, nil
}

func (s *Server) listTokenTransfers(r *http.Request) (interface{}, error) {
	tokenID := chi.URLParam(r, "token")

	var token web3.Address
	if err := token.UnmarshalText([]byte(tokenID)); err != nil {
		return nil, err
	}
	from, err := parseAddresses(r, "from")
	if err != nil {
		return nil, err
	}
	to, err := parseAddresses(r, "to")
	if err != nil {
		return nil, err
	}

	query := parsePagination(r, 100)
	filter := store.TransfersFilter{
		QueryPagination: query,
		Tokens: []web3.Address{
			token,
		},
		From: from,
		To:   to,
	}
	return s.store.GetTokenTransfers(filter)
}

func (s *Server) listFromTransfers(r *http.Request) (interface{}, error) {
	address := chi.URLParam(r, "address")

	var from web3.Address
	if err := from.UnmarshalText([]byte(address)); err != nil {
		return nil, err
	}

	tokens, err := parseAddresses(r, "tokens")
	if err != nil {
		return nil, err
	}
	to, err := parseAddresses(r, "to")
	if err != nil {
		return nil, err
	}

	query := parsePagination(r, 100)
	filter := store.TransfersFilter{
		QueryPagination: query,
		Tokens:          tokens,
		From:            []web3.Address{from},
		To:              to,
	}
	return s.store.GetTokenTransfers(filter)
}

func (s *Server) listToTransfers(r *http.Request) (interface{}, error) {
	address := chi.URLParam(r, "address")

	var to web3.Address
	if err := to.UnmarshalText([]byte(address)); err != nil {
		return nil, err
	}

	tokens, err := parseAddresses(r, "tokens")
	if err != nil {
		return nil, err
	}
	from, err := parseAddresses(r, "from")
	if err != nil {
		return nil, err
	}

	query := parsePagination(r, 100)
	filter := store.TransfersFilter{
		QueryPagination: query,
		Tokens:          tokens,
		To:              []web3.Address{to},
		From:            from,
	}
	return s.store.GetTokenTransfers(filter)
}
