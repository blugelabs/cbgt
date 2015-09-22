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
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"

	log "github.com/couchbase/clog"
	"github.com/couchbase/go-couchbase"
	"github.com/couchbase/go-couchbase/cbdatasource"
	"github.com/couchbase/gomemcached"
)

// DEST_EXTRAS_TYPE_DCP represents the extras that comes from DCP
// protocol.
const DEST_EXTRAS_TYPE_DCP = DestExtrasType(0x0002)

func init() {
	RegisterFeedType("couchbase", &FeedType{
		Start:         StartDCPFeed,
		Partitions:    CouchbasePartitions,
		PartitionSeqs: CouchbasePartitionSeqs,
		Public:        true,
		Description: "general/couchbase" +
			" - a Couchbase Server bucket will be the data source",
		StartSample: NewDCPFeedParams(),
	})
	RegisterFeedType("couchbase-dcp", &FeedType{
		Start:         StartDCPFeed,
		Partitions:    CouchbasePartitions,
		PartitionSeqs: CouchbasePartitionSeqs,
		Public:        false, // Won't be listed in /api/managerMeta output.
		Description: "general/couchbase-dcp" +
			" - a Couchbase Server bucket will be the data source," +
			" via DCP protocol",
		StartSample: NewDCPFeedParams(),
	})
}

// StartDCPFeed starts a DCP related feed and is registered at
// init/startup time with the system via RegisterFeedType().
func StartDCPFeed(mgr *Manager, feedName, indexName, indexUUID,
	sourceType, sourceName, bucketUUID, params string,
	dests map[string]Dest) error {
	server, poolName, bucketName :=
		CouchbaseParseSourceName(mgr.server, "default", sourceName)

	feed, err := NewDCPFeed(feedName, indexName, server, poolName,
		bucketName, bucketUUID, params, BasicPartitionFunc, dests,
		mgr.tagsMap != nil && !mgr.tagsMap["feed"])
	if err != nil {
		return fmt.Errorf("feed_dcp:"+
			" could not prepare DCP feed, server: %s,"+
			" bucketName: %s, indexName: %s, err: %v",
			mgr.server, bucketName, indexName, err)
	}
	err = feed.Start()
	if err != nil {
		return fmt.Errorf("feed_dcp:"+
			" could not start, server: %s, err: %v",
			mgr.server, err)
	}
	err = mgr.registerFeed(feed)
	if err != nil {
		feed.Close()
		return err
	}
	return nil
}

// A DCPFeed implements both Feed and cbdatasource.Receiver
// interfaces, and forwards any incoming cbdatasource.Receiver
// callbacks to the relevant, hooked-up Dest instances.
type DCPFeed struct {
	name       string
	indexName  string
	url        string
	poolName   string
	bucketName string
	bucketUUID string
	params     *DCPFeedParams
	pf         DestPartitionFunc
	dests      map[string]Dest
	disable    bool
	stopAfter  map[string]UUIDSeq // May be nil.
	bds        cbdatasource.BucketDataSource

	m       sync.Mutex // Protects the fields that follow.
	closed  bool
	lastErr error
	stats   *DestStats

	stopAfterReached map[string]bool // May be nil.
}

// DCPFeedParams are DCP data-source/feed specific connection
// parameters that may be part of a sourceParams JSON and is a
// superset of CBFeedParams.  DCPFeedParams holds the information used
// to populate a cbdatasource.BucketDataSourceOptions on calls to
// cbdatasource.NewBucketDataSource().  DCPFeedParams also implements
// the couchbase.AuthHandler interface.
type DCPFeedParams struct {
	AuthUser     string `json:"authUser"` // May be "" for no auth.
	AuthPassword string `json:"authPassword"`

	AuthSaslUser     string `json:"authSaslUser"` // May be "" for no auth.
	AuthSaslPassword string `json:"authSaslPassword"`

	// Factor (like 1.5) to increase sleep time between retries
	// in connecting to a cluster manager node.
	ClusterManagerBackoffFactor float32 `json:"clusterManagerBackoffFactor"`

	// Initial sleep time (millisecs) before first retry to cluster manager.
	ClusterManagerSleepInitMS int `json:"clusterManagerSleepInitMS"`

	// Maximum sleep time (millisecs) between retries to cluster manager.
	ClusterManagerSleepMaxMS int `json:"clusterManagerSleepMaxMS"`

	// Factor (like 1.5) to increase sleep time between retries
	// in connecting to a data manager node.
	DataManagerBackoffFactor float32 `json:"dataManagerBackoffFactor"`

	// Initial sleep time (millisecs) before first retry to data manager.
	DataManagerSleepInitMS int `json:"dataManagerSleepInitMS"`

	// Maximum sleep time (millisecs) between retries to data manager.
	DataManagerSleepMaxMS int `json:"dataManagerSleepMaxMS"`

	// Buffer size in bytes provided for UPR flow control.
	FeedBufferSizeBytes uint32 `json:"feedBufferSizeBytes"`

	// Used for UPR flow control and buffer-ack messages when this
	// percentage of FeedBufferSizeBytes is reached.
	FeedBufferAckThreshold float32 `json:"feedBufferAckThreshold"`
}

// NewDCPFeedParams returns a DCPFeedParams initialized with default
// values.
func NewDCPFeedParams() *DCPFeedParams {
	return &DCPFeedParams{
		ClusterManagerSleepMaxMS: 20000,
		DataManagerSleepMaxMS:    20000,
	}
}

// GetCredentials is part of the couchbase.AuthSaslHandler interface.
func (d *DCPFeedParams) GetCredentials() (string, string, string) {
	// TODO: bucketName not necessarily userName.
	return d.AuthUser, d.AuthPassword, d.AuthUser
}

type DCPFeedParamsSasl struct {
	DCPFeedParams
}

func (d *DCPFeedParamsSasl) GetSaslCredentials() (string, string) {
	return d.AuthSaslUser, d.AuthSaslPassword
}

// NewDCPFeed creates a new, ready-to-be-started DCP feed.
func NewDCPFeed(name, indexName, url, poolName,
	bucketName, bucketUUID, paramsStr string,
	pf DestPartitionFunc, dests map[string]Dest,
	disable bool) (*DCPFeed, error) {
	params := NewDCPFeedParams()
	stopAfter := StopAfterSourceParams{}

	if paramsStr != "" {
		err := json.Unmarshal([]byte(paramsStr), params)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(paramsStr), &stopAfter)
		if err != nil {
			return nil, err
		}
	}

	vbucketIds, err := ParsePartitionsToVBucketIds(dests)
	if err != nil {
		return nil, err
	}
	if len(vbucketIds) <= 0 {
		vbucketIds = nil
	}

	urls := strings.Split(url, ";")

	var auth couchbase.AuthHandler = params

	if params.AuthUser == "" &&
		params.AuthSaslUser == "" {
		for _, serverUrl := range urls {
			cbAuthHandler, err := NewCbAuthHandler(serverUrl)
			if err != nil {
				continue
			}
			params.AuthUser, params.AuthPassword, err =
				cbAuthHandler.GetCredentials()
			if err != nil {
				continue
			}
			params.AuthSaslUser, params.AuthSaslPassword, err =
				cbAuthHandler.GetSaslCredentials()
			if err != nil {
				continue
			}
			break
		}
	} else if params.AuthSaslUser != "" {
		auth = &DCPFeedParamsSasl{*params}
	}

	options := &cbdatasource.BucketDataSourceOptions{
		Name: fmt.Sprintf("%s-%x", name, rand.Int31()),
		ClusterManagerBackoffFactor: params.ClusterManagerBackoffFactor,
		ClusterManagerSleepInitMS:   params.ClusterManagerSleepInitMS,
		ClusterManagerSleepMaxMS:    params.ClusterManagerSleepMaxMS,
		DataManagerBackoffFactor:    params.DataManagerBackoffFactor,
		DataManagerSleepInitMS:      params.DataManagerSleepInitMS,
		DataManagerSleepMaxMS:       params.DataManagerSleepMaxMS,
		FeedBufferSizeBytes:         params.FeedBufferSizeBytes,
		FeedBufferAckThreshold:      params.FeedBufferAckThreshold,
	}

	feed := &DCPFeed{
		name:       name,
		indexName:  indexName,
		url:        url,
		poolName:   poolName,
		bucketName: bucketName,
		bucketUUID: bucketUUID,
		params:     params,
		pf:         pf,
		dests:      dests,
		disable:    disable,
		stopAfter:  stopAfter.StopAfterPartitionSeqs,
		stats:      NewDestStats(),
	}

	feed.bds, err = cbdatasource.NewBucketDataSource(
		urls, poolName, bucketName, bucketUUID,
		vbucketIds, auth, feed, options)
	if err != nil {
		return nil, err
	}

	return feed, nil
}

func (t *DCPFeed) Name() string {
	return t.name
}

func (t *DCPFeed) IndexName() string {
	return t.indexName
}

func (t *DCPFeed) Start() error {
	if t.disable {
		log.Printf("feed_dcp: disable, name: %s", t.Name())
		return nil
	}

	log.Printf("feed_dcp: start, name: %s", t.Name())
	return t.bds.Start()
}

func (t *DCPFeed) Close() error {
	t.m.Lock()
	if t.closed {
		t.m.Unlock()
		return nil
	}
	t.closed = true
	t.m.Unlock()

	log.Printf("feed_dcp: close, name: %s", t.Name())
	return t.bds.Close()
}

func (t *DCPFeed) Dests() map[string]Dest {
	return t.dests
}

var prefixBucketDataSourceStats = []byte(`{"bucketDataSourceStats":`)
var prefixDestStats = []byte(`,"destStats":`)

func (t *DCPFeed) Stats(w io.Writer) error {
	bdss := cbdatasource.BucketDataSourceStats{}
	err := t.bds.Stats(&bdss)
	if err != nil {
		return err
	}
	w.Write(prefixBucketDataSourceStats)
	json.NewEncoder(w).Encode(&bdss)

	w.Write(prefixDestStats)
	t.stats.WriteJSON(w)

	_, err = w.Write(JsonCloseBrace)
	return err
}

// --------------------------------------------------------

// checkStopAfter checks to see if we've already reached the
// stopAfterReached state for a partition.
func (r *DCPFeed) checkStopAfter(partition string) bool {
	r.m.Lock()
	reached := r.stopAfterReached != nil && r.stopAfterReached[partition]
	r.m.Unlock()

	return reached
}

// updateStopAfter checks and maintains the stopAfterReached tracking
// maps, which are used for so-called "one-time indexing".  Once we've
// reached the stopping point, we close the feed (after all partitions
// have reached their stopAfter sequence numbers).
func (r *DCPFeed) updateStopAfter(partition string, seq uint64) {
	if r.stopAfter == nil {
		return
	}

	uuidSeq, exists := r.stopAfter[partition]
	if !exists {
		return
	}

	// TODO: check UUID matches?
	if seq >= uuidSeq.Seq {
		allDone := false

		r.m.Lock()

		if r.stopAfterReached == nil {
			r.stopAfterReached = map[string]bool{}
		}
		r.stopAfterReached[partition] = true

		allDone = len(r.stopAfterReached) >= len(r.stopAfter)

		r.m.Unlock()

		if allDone {
			go r.Close()
		}
	}
}

// --------------------------------------------------------

func (r *DCPFeed) OnError(err error) {
	// TODO: Check the type of the error if it's something
	// serious / not-recoverable / needs user attention.
	log.Printf("feed_dcp: OnError, name: %s:"+
		" bucketName: %s, bucketUUID: %s, err: %v\n",
		r.name, r.bucketName, r.bucketUUID, err)

	atomic.AddUint64(&r.stats.TotError, 1)

	r.m.Lock()
	r.lastErr = err
	r.m.Unlock()
}

func (r *DCPFeed) DataUpdate(vbucketId uint16, key []byte, seq uint64,
	req *gomemcached.MCRequest) error {
	return Timer(func() error {
		partition, dest, err :=
			VBucketIdToPartitionDest(r.pf, r.dests, vbucketId, key)
		if err != nil || r.checkStopAfter(partition) {
			return err
		}

		err = dest.DataUpdate(partition, key, seq, req.Body,
			req.Cas, DEST_EXTRAS_TYPE_DCP, req.Extras)
		if err != nil {
			return fmt.Errorf("feed_dcp: DataUpdate,"+
				" name: %s, partition: %s, key: %s, seq: %d, err: %v",
				r.name, partition, key, seq, err)
		}

		r.updateStopAfter(partition, seq)

		return nil
	}, r.stats.TimerDataUpdate)
}

func (r *DCPFeed) DataDelete(vbucketId uint16, key []byte, seq uint64,
	req *gomemcached.MCRequest) error {
	return Timer(func() error {
		partition, dest, err :=
			VBucketIdToPartitionDest(r.pf, r.dests, vbucketId, key)
		if err != nil || r.checkStopAfter(partition) {
			return err
		}

		err = dest.DataDelete(partition, key, seq,
			req.Cas, DEST_EXTRAS_TYPE_DCP, req.Extras)
		if err != nil {
			return fmt.Errorf("feed_dcp: DataDelete,"+
				" name: %s, partition: %s, key: %s, seq: %d, err: %v",
				r.name, partition, key, seq, err)
		}

		r.updateStopAfter(partition, seq)

		return nil
	}, r.stats.TimerDataDelete)
}

func (r *DCPFeed) SnapshotStart(vbucketId uint16,
	snapStart, snapEnd uint64, snapType uint32) error {
	return Timer(func() error {
		partition, dest, err :=
			VBucketIdToPartitionDest(r.pf, r.dests, vbucketId, nil)
		if err != nil || r.checkStopAfter(partition) {
			return err
		}

		if r.stopAfter != nil {
			uuidSeq, exists := r.stopAfter[partition]
			if exists && snapEnd > uuidSeq.Seq { // TODO: Check UUID.
				// Clamp the snapEnd so batches are executed.
				snapEnd = uuidSeq.Seq
			}
		}

		return dest.SnapshotStart(partition, snapStart, snapEnd)
	}, r.stats.TimerSnapshotStart)
}

func (r *DCPFeed) SetMetaData(vbucketId uint16, value []byte) error {
	return Timer(func() error {
		partition, dest, err :=
			VBucketIdToPartitionDest(r.pf, r.dests, vbucketId, nil)
		if err != nil || r.checkStopAfter(partition) {
			return err
		}

		return dest.OpaqueSet(partition, value)
	}, r.stats.TimerOpaqueSet)
}

func (r *DCPFeed) GetMetaData(vbucketId uint16) (
	value []byte, lastSeq uint64, err error) {
	err = Timer(func() error {
		partition, dest, err :=
			VBucketIdToPartitionDest(r.pf, r.dests, vbucketId, nil)
		if err != nil || r.checkStopAfter(partition) {
			return err
		}

		value, lastSeq, err = dest.OpaqueGet(partition)

		return err
	}, r.stats.TimerOpaqueGet)

	return value, lastSeq, err
}

func (r *DCPFeed) Rollback(vbucketId uint16, rollbackSeq uint64) error {
	return Timer(func() error {
		partition, dest, err :=
			VBucketIdToPartitionDest(r.pf, r.dests, vbucketId, nil)
		if err != nil || r.checkStopAfter(partition) {
			return err
		}

		opaqueValue, lastSeq, err := dest.OpaqueGet(partition)
		if err != nil {
			return err
		}

		log.Printf("feed_dcp: rollback, name: %s: vbucketId: %d,"+
			" rollbackSeq: %d, partition: %s, opaqueValue: %s, lastSeq: %d",
			r.name, vbucketId, rollbackSeq,
			partition, opaqueValue, lastSeq)

		return dest.Rollback(partition, rollbackSeq)
	}, r.stats.TimerRollback)
}

// -------------------------------------------------------

type VBucketMetaData struct {
	FailOverLog [][]uint64 `json:"failOverLog"`
}

func ParseOpaqueToUUID(b []byte) string {
	vmd := &VBucketMetaData{}
	err := json.Unmarshal(b, &vmd)
	if err != nil {
		return ""
	}

	flogLen := len(vmd.FailOverLog)
	if flogLen < 1 || len(vmd.FailOverLog[flogLen-1]) < 1 {
		return ""
	}

	return fmt.Sprintf("%d", vmd.FailOverLog[flogLen-1][0])
}
