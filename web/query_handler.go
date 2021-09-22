package web

import (
	"encoding/json"
	"fmt"
	"github.com/crazycs520/continuous-profile/util/logutil"
	"go.uber.org/zap"
	"io"
	"net/http"
)

type Config struct {
	Enable        bool `json:"enable"`
	DurationSecs  uint `json:"duration_secs"`
	IntervalSecs  uint `json:"interval_secs"`
	RetentionSecs uint `json:"retention_secs"`
}

type BasicQueryParam struct {
	Begin   int64        `json:"begin_time"`
	End     int64        `json:"end_time"`
	Limit   int          `json:"limit"`
	Targets []TargetNode `json:"targets"`
}

type TargetNode struct {
	Job     string `json:"job"`
	Address string `json:"address"`
	Type    string `json:"type"`
}

func (s *Server) handleQueryList(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		break
	default:
		serveError(w, http.StatusBadRequest, "only support post")
		return
	}

}

func (s *Server) getQueryParamFromBody(r *http.Request) (*BasicQueryParam, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, nil
	}
	param := &BasicQueryParam{}
	err = json.Unmarshal(body, param)
	if err != nil {
		return nil, err
	}
	return param, nil
}

func serveError(w http.ResponseWriter, status int, txt string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Del("Content-Disposition")
	w.WriteHeader(status)
	_, err := fmt.Fprintln(w, txt)
	if err != nil {
		logutil.BgLogger().Error("serve error", zap.Error(err))
	}
}
