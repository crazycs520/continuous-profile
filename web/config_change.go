package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/crazycs520/continuous-profile/config"
	"github.com/crazycs520/continuous-profile/util/logutil"
	"go.uber.org/zap"
)

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
	default:
		serveError(w, http.StatusBadRequest, "only support post")
		return
	}
	err := s.handleConfigModify(w, r)
	if err != nil {
		logutil.BgLogger().Info("handle continuous profiling config modify failed", zap.Error(err))
		serveError(w, http.StatusBadRequest, "parse query param error: "+err.Error())
		return
	}
}

func (s *Server) handleConfigModify(w http.ResponseWriter, r *http.Request) error {
	var reqNested map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&reqNested); err != nil {
		return err
	}
	cfg := config.GetGlobalConfig()
	current, err := json.Marshal(cfg.ContinueProfiling)
	if err != nil {
		return err
	}

	var currentNested map[string]interface{}
	if err := json.NewDecoder(bytes.NewReader(current)).Decode(&currentNested); err != nil {
		return err
	}

	for k, v := range reqNested {
		old, ok := currentNested[k]
		if !ok {
			return fmt.Errorf("unknow config `%v`", k)
		}
		currentNested[k] = v
		logutil.BgLogger().Info("handle continuous profiling config modify",
			zap.String("name", k),
			zap.Reflect("old-value", old),
			zap.Reflect("new-value", v))
	}

	data, err := json.Marshal(currentNested)
	if err != err {
		return err
	}
	var newCfg config.ContinueProfilingConfig
	err = json.NewDecoder(bytes.NewReader(data)).Decode(&newCfg)
	if err != nil {
		return err
	}

	cfg.ContinueProfiling = newCfg
	config.StoreGlobalConfig(cfg)
	writeData(w, "success!")
	return nil
}
