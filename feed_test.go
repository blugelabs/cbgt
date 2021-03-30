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
	"bytes"
	"fmt"
	"io"
	"testing"
)

type ErrorOnlyFeed struct {
	name string
}

func (t *ErrorOnlyFeed) Name() string {
	return t.name
}

func (t *ErrorOnlyFeed) IndexName() string {
	return t.name
}

func (t *ErrorOnlyFeed) Start() error {
	return fmt.Errorf("ErrorOnlyFeed Start() invoked")
}

func (t *ErrorOnlyFeed) Close() error {
	return fmt.Errorf("ErrorOnlyFeed Close() invoked")
}

func (t *ErrorOnlyFeed) Dests() map[string]Dest {
	return nil
}

func (t *ErrorOnlyFeed) Stats(w io.Writer) error {
	return fmt.Errorf("ErrorOnlyFeed Stats() invoked")
}

func TestDataSourcePartitions(t *testing.T) {
	a, err := DataSourcePartitions("a fake source type",
		"sourceName", "sourceUUID", "sourceParams", "serverURL", nil)
	if err == nil || a != nil {
		t.Errorf("expected fake data source type to error")
	}

	a, err = DataSourcePartitions("nil",
		"sourceName", "sourceUUID", "sourceParams", "serverURL", nil)
	if err != nil || a != nil {
		t.Errorf("expected nil source type to work, but have no partitions")
	}

	a, err = DataSourcePartitions("primary",
		"sourceName", "sourceUUID", "sourceParams", "serverURL", nil)
	if err == nil || a != nil {
		t.Errorf("expected dest source type to error on non-json server params")
	}

	a, err = DataSourcePartitions("primary",
		"sourceName", "sourceUUID", "", "serverURL", nil)
	if err != nil || a == nil {
		t.Errorf("expected dest source type to ok on empty server params")
	}

	a, err = DataSourcePartitions("primary",
		"sourceName", "sourceUUID", "{}", "serverURL", nil)
	if err != nil || a == nil {
		t.Errorf("expected dest source type to ok on empty JSON server params")
	}
}

func TestNilFeedStart(t *testing.T) {
	f := NewNILFeed("aaa", "bbb", nil)
	if f.Name() != "aaa" {
		t.Errorf("expected aaa name")
	}
	if f.IndexName() != "bbb" {
		t.Errorf("expected bbb index name")
	}
	if f.Start() != nil {
		t.Errorf("expected NILFeed.Start() to work")
	}
	if f.Dests() != nil {
		t.Errorf("expected nil dests")
	}
	w := bytes.NewBuffer(nil)
	if f.Stats(w) != nil {
		t.Errorf("expected no err on nil feed stats")
	}
	if w.String() != "{}" {
		t.Errorf("expected json stats")
	}
	if f.Close() != nil {
		t.Errorf("expected nil dests")
	}
}

func TestPrimaryFeed(t *testing.T) {
	df := NewPrimaryFeed("aaa", "bbb",
		BasicPartitionFunc, map[string]Dest{})
	if df.Name() != "aaa" {
		t.Errorf("expected aaa name")
	}
	if df.IndexName() != "bbb" {
		t.Errorf("expected bbb index name")
	}
	if df.Start() != nil {
		t.Errorf("expected PrimaryFeed start to work")
	}

	buf := make([]byte, 0, 100)
	err := df.Stats(bytes.NewBuffer(buf))
	if err != nil {
		t.Errorf("expected PrimaryFeed stats to work")
	}

	key := []byte("k")
	seq := uint64(123)
	val := []byte("v")

	if df.DataUpdate("unknown-partition", key, seq, val,
		0, DEST_EXTRAS_TYPE_NIL, nil) == nil {
		t.Errorf("expected err on bad partition")
	}
	if df.DataDelete("unknown-partition", key, seq,
		0, DEST_EXTRAS_TYPE_NIL, nil) == nil {
		t.Errorf("expected err on bad partition")
	}
	if df.SnapshotStart("unknown-partition", seq, seq) == nil {
		t.Errorf("expected err on bad partition")
	}
	if df.OpaqueSet("unknown-partition", val) == nil {
		t.Errorf("expected err on bad partition")
	}
	_, _, err = df.OpaqueGet("unknown-partition")
	if err == nil {
		t.Errorf("expected err on bad partition")
	}
	if df.Rollback("unknown-partition", seq) == nil {
		t.Errorf("expected err on bad partition")
	}
	if df.ConsistencyWait("unknown-partition", "unknown-partition-UUID",
		"level", seq, nil) == nil {
		t.Errorf("expected err on bad partition")
	}
	df2 := NewPrimaryFeed("", "", BasicPartitionFunc, map[string]Dest{
		"some-partition": &TestDest{},
	})
	if df2.ConsistencyWait("some-partition", "some-partition-UUID",
		"level", seq, nil) != nil {
		t.Errorf("expected no err on some partition to TestDest")
	}
	_, err = df.Count(nil, nil)
	if err == nil {
		t.Errorf("expected err on counting a primary feed")
	}
	if df.Query(nil, nil, nil, nil) == nil {
		t.Errorf("expected err on querying a primary feed")
	}
}

func TestDataSourcePrepParams(t *testing.T) {
	a, err := DataSourcePrepParams("a fake source type",
		"sourceName", "sourceUUID", "sourceParams", "serverURL", nil)
	if err == nil || a != "" {
		t.Errorf("expected fake data source type to error")
	}

	a, err = DataSourcePrepParams("primary",
		"sourceName", "sourceUUID", "", "serverURL", nil)
	if err != nil || a != "" {
		t.Errorf("expected empty data source params to ok")
	}

	a, err = DataSourcePrepParams("primary",
		"sourceName", "sourceUUID", "{}", "serverURL", nil)
	if err != nil || a != "{}" {
		t.Errorf("expected {} data source params to ok")
	}

	saw_testFeedPartitions := 0
	saw_testFeedPartitionSeqs := 0

	testFeedPartitions := func(sourceType,
		sourceName, sourceUUID, sourceParams,
		serverIn string, options map[string]string,
	) (
		partitions []string, err error,
	) {
		saw_testFeedPartitions++
		return nil, nil
	}

	testFeedPartitionSeqs := func(sourceType, sourceName, sourceUUID,
		sourceParams, serverIn string, options map[string]string,
	) (
		map[string]UUIDSeq, error,
	) {
		saw_testFeedPartitionSeqs++
		return nil, nil
	}

	RegisterFeedType("testFeed", &FeedType{
		Partitions:    testFeedPartitions,
		PartitionSeqs: testFeedPartitionSeqs,
	})

	sourceParams := `{"foo":"hoo","markPartitionSeqs":"currentPartitionSeqs"}`
	a, err = DataSourcePrepParams("testFeed",
		"sourceName", "sourceUUID", sourceParams, "serverURL", nil)
	if err != nil {
		t.Errorf("expected no err")
	}
	if a == sourceParams {
		t.Errorf("expected transformed data source params")
	}
	if saw_testFeedPartitions != 1 {
		t.Errorf("expected 1 saw_testFeedPartitions call, got: %d",
			saw_testFeedPartitions)
	}
	if saw_testFeedPartitionSeqs != 1 {
		t.Errorf("expected 1 saw_testFeedPartitionSeqs call, got: %d",
			saw_testFeedPartitionSeqs)
	}
}
