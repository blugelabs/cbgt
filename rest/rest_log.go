//  Copyright (c) 2014 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the
//  License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing,
//  software distributed under the License is distributed on an "AS
//  IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
//  express or implied. See the License for the specific language
//  governing permissions and limitations under the License.

package rest

import (
	"encoding/json"
	"net/http"

	"github.com/blugelabs/cbgt"
)

// TODO: Need to give the codebase a scrub of its log
// messages and fmt.Errorf()'s.

// LogGetHandler is a REST handler that retrieves recent log messages.
type LogGetHandler struct {
	mgr *cbgt.Manager
	mr  *cbgt.MsgRing
}

func NewLogGetHandler(
	mgr *cbgt.Manager, mr *cbgt.MsgRing) *LogGetHandler {
	return &LogGetHandler{mgr: mgr, mr: mr}
}

func (h *LogGetHandler) ServeHTTP(
	w http.ResponseWriter, req *http.Request) {
	w.Write([]byte(`{"messages":[`))
	if h.mr != nil {
		for i, message := range h.mr.Messages() {
			buf, err := json.Marshal(string(message))
			if err == nil {
				if i > 0 {
					w.Write(cbgt.JsonComma)
				}
				w.Write(buf)
			}
		}
	}
	w.Write([]byte(`],"events":[`))
	if h.mgr != nil {
		first := true
		h.mgr.VisitEvents(func(event []byte) {
			if !first {
				w.Write(cbgt.JsonComma)
			}
			first = false
			w.Write(event)
		})
	}
	w.Write([]byte(`]}`))
}
