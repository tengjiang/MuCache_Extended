package cm

import (
	"github.com/DKW2/MuCache_Extended/pkg/cm"
	"github.com/DKW2/MuCache_Extended/pkg/common"
	"time"
	"github.com/golang/glog"
)

// These two functions do the communication with the caller cache manager
func sendSaveToCaller(cfg *Config, caller cm.ServiceName, callArgs cm.CallArgs, retVal cm.ReturnVal) {
	//glog.Infof(" -- Sending to: %v (Save-Cache-Line, ca: %v -> %v)\n", caller, callArgs, retVal)
	req := cm.SaveCallsRequest{CallArgsList: []cm.CallArgs{callArgs}, ReturnVals: []cm.ReturnVal{retVal}}
	request := HttpSendSaveCallsRequest{
		Request: req,
		Caller:  caller,
	}
	HttpQueue <- request
}

func sendInvsToCallers(cfg *Config, saved CallsAndCallers) {
	//glog.Infof(" -- Sending Invalidate line: %v\n", saved)
	request := HttpSendInvalidateCallRequest{InvSet: saved.Dict}
	HttpQueue <- request
}

func Process(cfg *Config, state *State) {

	// Batched Invalidation
	glog.Info( "Batched Invalidation is %v", cfg.batchCallInvalidation )
	invalidateBuffer := make(chan cm.InvalidateCallsRequest, 100) // Buffered channel
	if( cfg.batchCallInvalidation ) {
		ticker := time.NewTicker(100 * time.Millisecond)             // Process every 500ms
		defer ticker.Stop()

		go func() {
			glog.Info("Batched invalidation ticker goroutine started")
			for range ticker.C {
				glog.Info("Batched invalidation ticker fired")
				processInvalidateBuffer(cfg, state, invalidateBuffer)
			}
			glog.Info("Batched invalidation ticker goroutine stopped (unexpected)")
		}()
	}

	//profileState := initProfile(cfg.printTimeFreq)
	for {
		//if len(cm.WQ) >= cm.QueueSize/2 {
		//	glog.Warningf("Queue size: %v\n", len(cm.WQ))
		//}
		//profileState.profileProcRequest()
		request := <-cm.WQ
		switch request.(type) {
		case cm.StartRequest:
			startRequest := request.(cm.StartRequest)
			//glog.Infof("Processing: %v\n", &startRequest)
			state.appendCall(startRequest.CallArgs)
		case cm.EndRequest:
			endRequest := request.(cm.EndRequest)
			//glog.Infof("Processing: %v\n", &endRequest)
			if state.validCall(endRequest.CallArgs, endRequest.KeyDeps, endRequest.CallDeps) {
				sendSaveToCaller(cfg, endRequest.Caller, endRequest.CallArgs, endRequest.ReturnVal)
				state.storeDeps(cfg, endRequest.Caller, endRequest.CallArgs, endRequest.KeyDeps, endRequest.CallDeps)
			} else {
				// Only for debugging
				//glog.Infof("Call", endRequest.CallArgs, "not valid!")
			}
		case cm.InvalidateKeyRequest:
			invRequest := request.(cm.InvalidateKeyRequest)
			if common.ShardEnabled && !invRequest.FromCM {
				neighbors := cfg.GetNeighbors()
				//glog.Infof("neighbors: %v\n", neighbors)
				invRequest.FromCM = true
				for _, neighbor := range neighbors {
					cm.SendInvRequestHttp(&invRequest, neighbor)
				}
			}
			//glog.Infof("Processing: %v\n", &invRequest)
			state.appendWrite(invRequest.Key)
			saved, exists := state.popKeyDeps(invRequest.Key)
			if exists {
				sendInvsToCallers(cfg, saved)
			}
		case cm.InvalidateCallsRequest:
			// Batch invalidation
			if( cfg.batchCallInvalidation ) {
				invRequest := request.(cm.InvalidateCallsRequest) // Explicit type assertion
				invalidateBuffer <- invRequest
			} else {
				invRequest := request.(cm.InvalidateCallsRequest)
				//glog.Infof("Processing: %v\n", &invRequest)
				for _, ca := range invRequest.Calls {
					state.appendInvCall(&ca)
				}

				// Transitively invalidate upstream entries
				callDeps, exists := state.popCallDeps(invRequest.Calls)
				if exists {
					sendInvsToCallers(cfg, callDeps)
				}

				// Remove the call entries from the local cache
				cm.CacheRemoveCalls(cfg.cacheClient, invRequest.Calls)
			}
		case cm.SaveCallsRequest:
			saveRequest := request.(cm.SaveCallsRequest)
			//glog.Infof("Processing: %v, Len: %v\n", &saveRequest, len(saveRequest.CallArgsList))
			cm.CacheSaveCalls(cfg.cacheClient, saveRequest.CallArgsList, saveRequest.ReturnVals)
		default:
			panic("Unreachable")
		}
		//fmt.Println("State after processing:", request)
		//fmt.Println(state.String())
	}
}

func processInvalidateBuffer(cfg *Config, state *State, buffer <-chan cm.InvalidateCallsRequest) {
	glog.Infof("Doing Batched Invalidation")
	mergedCalls := make(map[cm.CallArgs]bool)
	requestsToProcess := []cm.InvalidateCallsRequest{}
	flushCount := 0 // Add a counter to track flushes

	// Drain the buffer as much as possible in this invocation
	for {
		select {
		case req := <-buffer:
			requestsToProcess = append(requestsToProcess, req)
			for _, call := range req.Calls {
				mergedCalls[call] = true
			}
			flushCount++ // Increment the flush counter
			glog.Infof("Buffer flush iteration: %d, Requests in batch: %d, Merged calls: %d", flushCount, len(requestsToProcess), len(mergedCalls)) // Added log
		default:
			glog.Info("Buffer drain finished for this invocation")
			goto MergedProcessing // Exit the draining loop
		}
	}

MergedProcessing:
	if len(mergedCalls) > 0 {
		mergedCallSlice := make([]cm.CallArgs, 0, len(mergedCalls))
		for call := range mergedCalls {
			mergedCallSlice = append(mergedCallSlice, call)
		}

		for _, ca := range mergedCallSlice {
			state.appendInvCall(&ca)
		}

		callDeps, exists := state.popCallDeps(mergedCallSlice)
		if exists {
			sendInvsToCallers(cfg, callDeps)
		}

		cm.CacheRemoveCalls(cfg.cacheClient, mergedCallSlice)
		glog.Infof("Processed batched invalidation of %d calls in this batch", len(mergedCallSlice))
	} else {
		glog.Info("No calls to process in buffer during this invocation")
	}
	glog.Info("processInvalidateBuffer finished")
}
