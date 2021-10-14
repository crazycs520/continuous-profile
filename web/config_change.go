package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/crazycs520/continuous-profile/config"
	"github.com/pingcap/log"
	"go.uber.org/zap"
)

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := config.GetGlobalConfig()
		writeData(w, cfg)
		return
	case http.MethodPost:
	default:
		serveError(w, http.StatusBadRequest, "only support post")
		return
	}
	err := s.handleConfigModify(w, r)
	if err != nil {
		log.Info("handle config modify failed", zap.Error(err))
		serveError(w, http.StatusBadRequest, "parse query param error: "+err.Error())
		return
	}
}

func (s *Server) handleConfigModify(w http.ResponseWriter, r *http.Request) error {
	var reqNested map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&reqNested); err != nil {
		return err
	}
	for k, v := range reqNested {
		switch k {
		case "continuous_profiling":
			m, ok := v.(map[string]interface{})
			if !ok {
				return fmt.Errorf("%v config value is invalid: %v", k, v)
			}
			return s.handleContinueProfilingConfigModify(w, m)
		default:
			return fmt.Errorf("config %v not support modify or unknow", k)
		}
	}
	return nil
}

func (s *Server) handleContinueProfilingConfigModify(w http.ResponseWriter, reqNested map[string]interface{}) error {
	cfg := config.GetGlobalConfig()
	current, err := json.Marshal(cfg.ContinueProfiling)
	if err != nil {
		return err
	}

	var currentNested map[string]interface{}
	if err := json.NewDecoder(bytes.NewReader(current)).Decode(&currentNested); err != nil {
		return err
	}

	for k, newValue := range reqNested {
		oldValue, ok := currentNested[k]
		if !ok {
			return fmt.Errorf("unknow config `%v`", k)
		}
		if oldValue == newValue {
			continue
		}
		currentNested[k] = newValue
		log.Info("handle continuous profiling config modify",
			zap.String("name", k),
			zap.Reflect("old-value", oldValue),
			zap.Reflect("new-value", newValue))
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
	s.scraper.NotifyReload()
	writeData(w, "success!")
	return nil
}
