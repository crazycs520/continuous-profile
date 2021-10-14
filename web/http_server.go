package web

import (
	"fmt"
	"github.com/crazycs520/continuous-profile/scrape"
	"github.com/crazycs520/continuous-profile/store"
	"github.com/crazycs520/continuous-profile/util"
	"github.com/crazycs520/continuous-profile/util/logutil"
	"go.uber.org/zap"
	"net"
	"net/http"
	"net/http/pprof"

	"github.com/gorilla/mux"
)

type Server struct {
	address    string
	httpServer *http.Server
	store      *store.ProfileStorage
	scraper    *scrape.Manager
}

func CreateHTTPServer(host string, port uint, store *store.ProfileStorage, scraper *scrape.Manager) *Server {
	return &Server{
		address: fmt.Sprintf("%v:%v", host, port),
		store:   store,
		scraper: scraper,
	}
}

func (s *Server) StartServer() error {
	serverMux := s.createMux()
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return err
	}
	s.httpServer = &http.Server{
		Addr:    s.address,
		Handler: serverMux,
	}
	go util.GoWithRecovery(func() {
		err = s.httpServer.Serve(listener)
		if err != nil {
			logutil.BgLogger().Error("http server serve failed", zap.Error(err))
		}
	}, nil)
	logutil.BgLogger().Info("http server started", zap.String("address", s.address))
	return nil
}

func (s *Server) Close() error {
	return s.httpServer.Close()
}

func (s *Server) createMux() *http.ServeMux {
	router := mux.NewRouter()
	router.HandleFunc("/config", s.handleConfig)

	// continuous profiling api
	router.HandleFunc("/continuous-profiling/list", s.handleQueryList)
	router.HandleFunc("/continuous-profiling/download", s.handleDownload)
	router.HandleFunc("/continuous-profiling/components", s.handleComponents)
	router.HandleFunc("/continuous-profiling/estimate_size", s.handleEstimateSize)

	serverMux := http.NewServeMux()
	serverMux.Handle("/", router)
	serverMux.HandleFunc("/debug/pprof/", pprof.Index)
	serverMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	serverMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	serverMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	serverMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return serverMux
}
