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

package cbgt

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// A Feed interface represents an abstract data source.  A Feed
// instance is hooked up to one-or-more Dest instances.  When incoming
// data is received by a Feed, the Feed will invoke relevant methods on
// the relevant Dest instances.
//
// In this codebase, the words "index source", "source" and "data
// source" are often associated with and used roughly as synonyms with
// "feed".
type Feed interface {
	Name() string
	IndexName() string
	Start() error
	Close() error
	Dests() map[string]Dest // Key is partition identifier.

	// Writes stats as JSON to the given writer.
	Stats(io.Writer) error
}

// Default values for feed parameters.
const FEED_SLEEP_MAX_MS = 10000
const FEED_SLEEP_INIT_MS = 100
const FEED_BACKOFF_FACTOR = 1.5

// FeedTypes is a global registry of available feed types and is
// initialized on startup.  It should be immutable after startup time.
var FeedTypes = make(map[string]*FeedType) // Key is sourceType.

// A FeedType represents an immutable registration of a single feed
// type or data source type.
type FeedType struct {
	Start            FeedStartFunc
	Partitions       FeedPartitionsFunc
	PartitionSeqs    FeedPartitionSeqsFunc    // Optional.
	Stats            FeedStatsFunc            // Optional.
	PartitionLookUp  FeedPartitionLookUpFunc  // Optional.
	SourceUUIDLookUp FeedSourceUUIDLookUpFunc // Optional.
	Public           bool
	Description      string
	StartSample      interface{}
	StartSampleDocs  map[string]string
}

// A FeedStartFunc is part of a FeedType registration as is invoked by
// a Manager when a new feed instance needs to be started.
type FeedStartFunc func(mgr *Manager,
	feedName, indexName, indexUUID string,
	sourceType, sourceName, sourceUUID, sourceParams string,
	dests map[string]Dest) error

// Each Feed or data-source type knows of the data partitions for a
// data source.
type FeedPartitionsFunc func(sourceType, sourceName, sourceUUID,
	sourceParams, server string,
	options map[string]string) ([]string, error)

// Returns the current partitions and their seq's.
type FeedPartitionSeqsFunc func(sourceType, sourceName, sourceUUID,
	sourceParams, server string,
	options map[string]string) (map[string]UUIDSeq, error)

// A UUIDSeq associates a UUID (such as from a partition's UUID) with
// a seq number.
type UUIDSeq struct {
	UUID string
	Seq  uint64
}

// Returns the current stats from a data source, if available,
// where the result is dependent on the data source / feed type.
type FeedStatsFunc func(sourceType, sourceName, sourceUUID,
	sourceParams, server string,
	options map[string]string,
	statsKind string) (map[string]interface{}, error)

// Performs a lookup of a source partition given a document id.
type FeedPartitionLookUpFunc func(docID, server string,
	sourceDetails *IndexDef,
	req *http.Request) (string, error)

// Returns the sourceUUID for a data source.
type FeedSourceUUIDLookUpFunc func(sourceName, sourceParams, server string,
	options map[string]string) (string, error)

// StopAfterSourceParams defines optional fields for the sourceParams
// that can stop the data source feed (i.e., index ingest) if the seqs
// per partition have been reached.  It can be used, for example, to
// help with "one-time indexing" behavior.
type StopAfterSourceParams struct {
	// Valid values: "", "markReached".
	StopAfter string `json:"stopAfter"`

	// Keyed by source partition.
	MarkPartitionSeqs map[string]UUIDSeq `json:"markPartitionSeqs"`
}

// RegisterFeedType is invoked at init/startup time to register a
// FeedType.
func RegisterFeedType(sourceType string, f *FeedType) {
	FeedTypes[sourceType] = f
}

// ------------------------------------------------------------------------

// dataSourcePartitions is a helper function that returns the data
// source partitions for a named data source or feed type.
func dataSourcePartitions(sourceType, sourceName, sourceUUID, sourceParams,
	server string, options map[string]string) ([]string, error) {
	feedType, exists := FeedTypes[sourceType]
	if !exists || feedType == nil {
		return nil, fmt.Errorf("feed: dataSourcePartitions"+
			" unknown sourceType: %s", sourceType)
	}

	return feedType.Partitions(sourceType, sourceName, sourceUUID,
		sourceParams, server, options)
}

// ------------------------------------------------------------------------

// dataSourcePrepParams parses and validates the sourceParams,
// possibly transforming it.  One transform is if the
// "markPartitionSeqs" field in the sourceParams has a string value of
// "currentPartitionSeqs", then the markPartitionSeqs will be
// transformed into a map[string]UUIDSeq.  dataSourcePrepParams
// returns the transformed sourceParams.
func dataSourcePrepParams(sourceType, sourceName, sourceUUID, sourceParams,
	server string, options map[string]string) (string, error) {
	_, err := dataSourcePartitions(sourceType, sourceName, sourceUUID,
		sourceParams, server, options)
	if err != nil {
		return "", err
	}

	if sourceParams == "" {
		return "", nil
	}

	feedType, exists := FeedTypes[sourceType]
	if !exists || feedType == nil {
		return "", fmt.Errorf("feed: dataSourcePrepParams"+
			" unknown sourceType: %s", sourceType)
	}

	if feedType.PartitionSeqs == nil {
		return sourceParams, nil
	}

	var sourceParamsMap map[string]interface{}
	err = json.Unmarshal([]byte(sourceParams), &sourceParamsMap)
	if err != nil {
		return "", fmt.Errorf("feed: dataSourcePrepParams"+
			" json parse sourceParams: %s, err: %v",
			sourceParams, err)
	}

	if sourceParamsMap != nil {
		v, exists := sourceParamsMap["markPartitionSeqs"]
		if exists {
			markPartitionSeqs, ok := v.(string)
			if ok && markPartitionSeqs == "currentPartitionSeqs" {
				partitionSeqs, err := feedType.PartitionSeqs(
					sourceType, sourceName, sourceUUID,
					sourceParams, server, options)
				if err != nil {
					return "", fmt.Errorf("feed: dataSourcePrepParams"+
						" PartitionSeqs, err: %v", err)
				}

				sourceParamsMap["markPartitionSeqs"] = partitionSeqs

				j, err := json.Marshal(sourceParamsMap)
				if err != nil {
					return "", err
				}

				sourceParams = string(j)
			}
		}
	}

	return sourceParams, nil
}

// ------------------------------------------------------------------------

// DataSourceUUID is a helper function that fetches the sourceUUID for
// the sourceName.
func DataSourceUUID(sourceType, sourceName, sourceParams, server string,
	options map[string]string) (string, error) {
	feedType, exists := FeedTypes[sourceType]
	if !exists || feedType == nil {
		return "", fmt.Errorf("feed: DataSourceUUID"+
			" unknown sourceType: %s", sourceType)
	}

	if feedType.SourceUUIDLookUp == nil {
		return "", nil
	}

	return feedType.SourceUUIDLookUp(sourceName, sourceParams, server, options)
}
