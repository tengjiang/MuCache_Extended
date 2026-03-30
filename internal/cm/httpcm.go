package cm

import (
	"github.com/goccy/go-json"
	"fmt"
	"github.com/DKW2/MuCache_Extended/pkg/cm"
	"github.com/DKW2/MuCache_Extended/pkg/common"
	"github.com/golang/glog"
	"net/http"
)

func startHandler(w http.ResponseWriter, req *http.Request) {
	var startReq cm.StartRequest
	err := json.NewDecoder(req.Body).Decode(&startReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	cm.WQ <- startReq
}

func endHandler(w http.ResponseWriter, req *http.Request) {
	var endReq cm.EndRequest
	err := json.NewDecoder(req.Body).Decode(&endReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	cm.WQ <- endReq
}

func invHandler(w http.ResponseWriter, req *http.Request) {
	var invReq cm.InvalidateKeyRequest
	err := json.NewDecoder(req.Body).Decode(&invReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	cm.WQ <- invReq
}

func invCallsHandler(w http.ResponseWriter, req *http.Request) {
	var request cm.InvalidateCallsRequest
	err := json.NewDecoder(req.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	cm.WQ <- request
}

func saveCallHandler(w http.ResponseWriter, req *http.Request) {
	var request cm.SaveCallsRequest
	err := json.NewDecoder(req.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	cm.WQ <- request
}

func ServeHttp(cfg *Config) {
	mux := http.NewServeMux()
	if !common.ZMQ {
		mux.HandleFunc(cm.HttpStartSuffix, startHandler)
		mux.HandleFunc(cm.HttpEndSuffix, endHandler)
	}
	mux.HandleFunc(cm.HttpInvSuffix, invHandler)
	mux.HandleFunc(cm.HttpInvCallsSuffix, invCallsHandler)
	mux.HandleFunc(cm.HttpSaveCallsSuffix, saveCallHandler)

	addr := fmt.Sprintf(":%d", cfg.port)
	glog.Infof("CM listening on %s", addr)
	panic(http.ListenAndServe(addr, mux))
}
