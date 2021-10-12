package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/crazycs520/continuous-profile/config"
	"net/http"
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
		serveError(w, http.StatusBadRequest, "parse query param error: "+err.Error())
		return
	}

	//param, err := s.getQueryParamFromBody(r)
	//if err != nil {
	//	serveError(w, http.StatusBadRequest, "parse query param error: "+err.Error())
	//	return
	//}
	//
	//result, err := s.store.QueryProfileList(param)
	//if err != nil {
	//	serveError(w, http.StatusInternalServerError, "query profile error: "+err.Error())
	//	return
	//}
	//writeData(w, result)
}

func (s *Server) handleConfigModify(w http.ResponseWriter, r *http.Request) error {
	var nested map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&nested); err != nil {
		return err
	}
	for k, v := range nested {
		switch k {
		case "continuous_profiling":
			return s.handleContinueProfilingConfigModify(w, v)
		default:
			return fmt.Errorf("config `%v` not support modified", k)
		}
	}

	fmt.Printf("req: %v \n", nested)
	return nil
}

func (s *Server) handleContinueProfilingConfigModify(w http.ResponseWriter, reqCfg interface{}) error {
	reqNested :=

	cfg := config.GetGlobalConfig().ContinueProfiling
	current, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	var currentNested map[string]interface{}
	if err := json.NewDecoder(bytes.NewReader(current)).Decode(&currentNested); err != nil {
		return err
	}
	fmt.Printf("cur: %v \n", currentNested)

	data, err := json.Marshal(currentNested)
	if err != err {
		return err
	}
	var newCfg config.ContinueProfilingConfig
	json.NewDecoder(bytes.NewReader(data)).Decode(&newCfg)
	fmt.Printf("new: %#v \n", newCfg)

}
