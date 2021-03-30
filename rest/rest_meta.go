//  Copyright (c) 2014 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
//  except in compliance with the License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software distributed under the
//  License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  either express or implied. See the License for the specific language governing permissions
//  and limitations under the License.

package rest

import (
	"net/http"

	"github.com/blugelabs/cbgt"
)

// ManagerMetaHandler is a REST handler that returns metadata about a
// manager/node.
type ManagerMetaHandler struct {
	mgr  *cbgt.Manager
	meta map[string]RESTMeta
}

func NewManagerMetaHandler(mgr *cbgt.Manager,
	meta map[string]RESTMeta) *ManagerMetaHandler {
	return &ManagerMetaHandler{mgr: mgr, meta: meta}
}

// MetaDesc represents a part of the JSON of a ManagerMetaHandler REST
// response.
type MetaDesc struct {
	Description     string            `json:"description"`
	StartSample     interface{}       `json:"startSample"`
	StartSampleDocs map[string]string `json:"startSampleDocs"`
}

// MetaDescSource represents the source-type/feed-type parts of the
// JSON of a ManagerMetaHandler REST response.
type MetaDescSource MetaDesc

// MetaDescSource represents the index-type parts of
// the JSON of a ManagerMetaHandler REST response.
type MetaDescIndex struct {
	MetaDesc

	CanCount bool `json:"canCount"`
	CanQuery bool `json:"canQuery"`

	QuerySamples interface{} `json:"querySamples"`
	QueryHelp    string      `json:"queryHelp"`

	UI map[string]string `json:"ui"`
}

func (h *ManagerMetaHandler) ServeHTTP(
	w http.ResponseWriter, req *http.Request) {
	ps := cbgt.IndexPartitionSettings(h.mgr)

	startSamples := map[string]interface{}{
		"planParams": &cbgt.PlanParams{
			MaxPartitionsPerPIndex: ps.MaxPartitionsPerPIndex,
			IndexPartitions:        ps.IndexPartitions,
		},
	}

	// Key is sourceType, value is description.
	sourceTypes := map[string]*MetaDescSource{}
	for sourceType, f := range cbgt.FeedTypes {
		if f.Public {
			sourceTypes[sourceType] = &MetaDescSource{
				Description:     f.Description,
				StartSample:     f.StartSample,
				StartSampleDocs: f.StartSampleDocs,
			}
		}
	}

	// Key is indexType, value is description.
	indexTypes := map[string]*MetaDescIndex{}
	for indexType, t := range cbgt.PIndexImplTypes {
		mdi := &MetaDescIndex{
			MetaDesc: MetaDesc{
				Description: t.Description,
				StartSample: t.StartSample,
			},
			CanCount:  t.Count != nil,
			CanQuery:  t.Query != nil,
			QueryHelp: t.QueryHelp,
			UI:        t.UI,
		}

		if t.QuerySamples != nil {
			mdi.QuerySamples = t.QuerySamples()
		}

		indexTypes[indexType] = mdi
	}

	r := map[string]interface{}{
		"status":       "ok",
		"startSamples": startSamples,
		"sourceTypes":  sourceTypes,
		"indexNameRE":  cbgt.INDEX_NAME_REGEXP,
		"indexTypes":   indexTypes,
		"refREST":      h.meta,
	}

	for _, t := range cbgt.PIndexImplTypes {
		if t.MetaExtra != nil {
			t.MetaExtra(r)
		}
	}

	MustEncode(w, r)
}
