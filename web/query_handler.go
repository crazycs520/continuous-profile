package web

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/crazycs520/continuous-profile/config"
	"github.com/crazycs520/continuous-profile/meta"
	"github.com/pingcap/log"
	"go.uber.org/zap"
)

type Config struct {
	Enable        bool `json:"enable"`
	DurationSecs  uint `json:"duration_secs"`
	IntervalSecs  uint `json:"interval_secs"`
	RetentionSecs uint `json:"retention_secs"`
}

func (s *Server) handleQueryList(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		break
	default:
		serveError(w, http.StatusBadRequest, "only support post")
		return
	}
	param, err := s.getQueryParamFromBody(r)
	if err != nil {
		serveError(w, http.StatusBadRequest, "parse query param error: "+err.Error())
		return
	}

	result, err := s.store.QueryProfileList(param)
	if err != nil {
		serveError(w, http.StatusInternalServerError, "query profile error: "+err.Error())
		return
	}
	writeData(w, result)
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		break
	default:
		serveError(w, http.StatusBadRequest, "only support post")
		return
	}
	param, err := s.getQueryParamFromBody(r)
	if err != nil {
		serveError(w, http.StatusBadRequest, "parse query param error: "+err.Error())
		return
	}

	w.Header().
		Set("Content-Disposition",
			fmt.Sprintf(`attachment; filename="profile"`+time.Now().Format("20060102150405")+".zip"))
	zw := zip.NewWriter(w)
	fn := func(pt meta.ProfileTarget, ts int64, data []byte) error {
		fileName := fmt.Sprintf("%v_%v_%v_%v", pt.Kind, pt.Component, pt.Address, ts)
		fw, err := zw.Create(fileName)
		if err != nil {
			return err
		}
		_, err = fw.Write(data)
		return err
	}

	err = s.store.QueryProfileData(param, fn)
	if err != nil {
		serveError(w, http.StatusInternalServerError, "query profile error: "+err.Error())
		return
	}
	err = zw.Close()
	if err != nil {
		log.Error("handle download request failed", zap.Error(err))
	}
}

func (s *Server) handleComponents(w http.ResponseWriter, r *http.Request) {
	components := s.scraper.GetCurrentScrapeComponents()
	writeData(w, components)
}

func (s *Server) handleEstimateSize(w http.ResponseWriter, r *http.Request) {
	days := 0
	if value := r.FormValue("days"); len(value) > 0 {
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			serveError(w, http.StatusInternalServerError, "params days value is invalid, should be int")
			return
		}
		days = int(v)
	}
	if days == 0 {
		writeData(w, 0)
		return
	}
	_, suites := s.scraper.GetAllCurrentScrapeSuite()
	totalSize := 0
	for _, suite := range suites {
		size := suite.LastScrapeSize()
		if size == 0 {
			size = 500 * 1024
		}
		totalSize += size
	}
	cfg := config.GetGlobalConfig().ContinueProfiling
	compressRatio := 10
	estimateSize := (days * 24 * 60 * 60 / cfg.IntervalSeconds) * totalSize / compressRatio
	writeData(w, estimateSize)
}

func (s *Server) getQueryParamFromBody(r *http.Request) (*meta.BasicQueryParam, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, nil
	}
	param := &meta.BasicQueryParam{}
	err = json.Unmarshal(body, param)
	if err != nil {
		return nil, err
	}
	return param, nil
}

const (
	headerContentType = "Content-Type"
	contentTypeJSON   = "application/json"
)

func writeData(w http.ResponseWriter, data interface{}) {
	js, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		serveError(w, http.StatusBadRequest, err.Error())
		return
	}
	// write response
	w.Header().Set(headerContentType, contentTypeJSON)
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(js)
	if err != nil {
		log.Error("write http response error", zap.Error(err))
	}
}

func serveError(w http.ResponseWriter, status int, txt string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Del("Content-Disposition")
	w.WriteHeader(status)
	_, err := fmt.Fprintln(w, txt)
	if err != nil {
		log.Error("serve error", zap.Error(err))
	}
}
