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
	"hash/crc32"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"testing"
	"time"
)

func TestFilesPathToPartition(t *testing.T) {
	h := crc32.NewIEEE()
	partitions := []string(nil)
	if FilesPathToPartition(h, partitions, "hi") != "" {
		t.Errorf("expected empty partition")
	}
	partitions = []string{"a"}
	if FilesPathToPartition(h, partitions, "hi") != "a" {
		t.Errorf("expected a partition")
	}
	partitions = []string{"a", "b", "c"}
	p0 := FilesPathToPartition(h, partitions, "hi")
	p1 := FilesPathToPartition(h, partitions, "hi")
	if p0 != p1 {
		t.Errorf("expected same partition")
	}
}

func TestFilesFindMatches(t *testing.T) {
	var modTimeGTE time.Time
	regExps := []string(nil)
	var maxSize int64

	paths, err := FilesFindMatches("does not exist", "nope",
		regExps, modTimeGTE, maxSize)
	if err == nil || len(paths) > 0 {
		t.Errorf("expected err")
	}

	testDir, _ := ioutil.TempDir("./tmp", "test")
	defer os.RemoveAll(testDir)
	err = os.MkdirAll(testDir+
		string(os.PathSeparator)+"files"+
		string(os.PathSeparator)+"foo", 0700)
	if err != nil {
		t.Errorf("mkdirall error")
	}

	paths, err = FilesFindMatches(testDir, "foo",
		regExps, modTimeGTE, maxSize)
	if err != nil {
		t.Errorf("expected no err, err: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected no paths from empty dir")
	}

	err = os.MkdirAll(testDir+
		string(os.PathSeparator)+"files"+
		string(os.PathSeparator)+"foo"+
		string(os.PathSeparator)+"bar", 0700)
	if err != nil {
		t.Errorf("mkdirall error")
	}

	paths, err = FilesFindMatches(testDir, "foo",
		regExps, modTimeGTE, maxSize)
	if err != nil {
		t.Errorf("expected no err, err: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected no paths from empty dir")
	}

	hiPath := testDir +
		string(os.PathSeparator) + "files" +
		string(os.PathSeparator) + "foo" +
		string(os.PathSeparator) + "hi.txt"
	ioutil.WriteFile(hiPath, []byte("hello world"), 0600)

	paths, err = FilesFindMatches(testDir, "foo",
		regExps, modTimeGTE, maxSize)
	if err != nil {
		t.Errorf("expected no err, err: %v", err)
	}
	if len(paths) != 1 || paths[0] != hiPath {
		t.Errorf("expected hiPath, paths: %v", paths)
	}

	byePath := testDir +
		string(os.PathSeparator) + "files" +
		string(os.PathSeparator) + "foo" +
		string(os.PathSeparator) + "bar" +
		string(os.PathSeparator) + "bye.md"
	ioutil.WriteFile(byePath, []byte("goodbye world"), 0600)

	paths, err = FilesFindMatches(testDir, "foo",
		regExps, modTimeGTE, maxSize)
	if err != nil {
		t.Errorf("expected no err, err: %v", err)
	}
	sort.Strings(paths)
	if len(paths) != 2 ||
		paths[0] != byePath ||
		paths[1] != hiPath {
		t.Errorf("expected hiPath & byePath, paths: %v", paths)
	}

	paths, err = FilesFindMatches(testDir, "foo",
		regExps, time.Now(), maxSize)
	if err != nil {
		t.Errorf("expected no err, err: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected no paths due to modTimeGTE time, paths: %v", paths)
	}

	paths, err = FilesFindMatches(testDir, "foo",
		regExps, modTimeGTE, 5)
	if err != nil {
		t.Errorf("expected no err, err: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected no paths due to small maxSize, paths: %v", paths)
	}

	paths, err = FilesFindMatches(testDir, "foo",
		[]string{".txt$"}, modTimeGTE, maxSize)
	if err != nil {
		t.Errorf("expected no err, err: %v", err)
	}
	sort.Strings(paths)
	if len(paths) != 1 ||
		paths[0] != hiPath {
		t.Errorf("expected hiPath only due to regexp, paths: %v", paths)
	}

	paths, err = FilesFindMatches(testDir, "foo",
		[]string{".json$"}, modTimeGTE, maxSize)
	if err != nil {
		t.Errorf("expected no err, err: %v", err)
	}
	sort.Strings(paths)
	if len(paths) != 0 {
		t.Errorf("expected no paths due to regexp, paths: %v", paths)
	}

	paths, err = FilesFindMatches(testDir, "foo",
		[]string{"$$[bogus regexp"}, modTimeGTE, maxSize)
	if err == nil {
		t.Errorf("expected err on bogus regexp")
	}
	if paths != nil {
		t.Errorf("expected nil paths on bogus regexp")
	}
}

func TestFilesFeedPartitions(t *testing.T) {
	sourceType := ""
	sourceName := ""
	sourceUUID := ""
	sourceParams := ""
	server := ""

	partitions, err := FilesFeedPartitions(sourceType, sourceName,
		sourceUUID, sourceParams, server, nil)
	if err != nil {
		t.Errorf("expected no err, err: %v", err)
	}
	if len(partitions) != 0 {
		t.Errorf("expected no partitions")
	}

	partitions, err = FilesFeedPartitions(sourceType, sourceName,
		sourceUUID, "this}{is]not[json", server, nil)
	if err == nil {
		t.Errorf("expected err on bad JSON")
	}
	if partitions != nil {
		t.Errorf("expected nil partitions on bad JSON")
	}

	partitions, err = FilesFeedPartitions(sourceType, sourceName,
		sourceUUID, `{"numPartitions":0}`, server, nil)
	if err != nil {
		t.Errorf("expected no err, err: %v", err)
	}
	if len(partitions) != 0 {
		t.Errorf("expected no partitions")
	}

	partitions, err = FilesFeedPartitions(sourceType, sourceName,
		sourceUUID, `{"numPartitions":1}`, server, nil)
	if err != nil {
		t.Errorf("expected no err, err: %v", err)
	}
	if len(partitions) != 1 {
		t.Errorf("expected 1 partitions")
	}

	partitions, err = FilesFeedPartitions(sourceType, sourceName,
		sourceUUID, `{"numPartitions":13}`, server, nil)
	if err != nil {
		t.Errorf("expected no err, err: %v", err)
	}
	if len(partitions) != 13 {
		t.Errorf("expected 13 partitions")
	}
}

func TestFilesFeedDisabled(t *testing.T) {
	params := ""
	dests := map[string]Dest{}
	l := NewStdLibLog(os.Stderr, "", log.LstdFlags)
	ff, err := NewFilesFeed(nil, "name", "indexName", "sourceName",
		params, dests, true, l)
	if err != nil {
		t.Errorf("expected no err, err: %v", err)
	}
	if ff == nil {
		t.Errorf("expected ff")
	}
	err = ff.Start()
	if err != nil {
		t.Errorf("expected disabled ff start to work")
	}
	if ff.IndexName() != "indexName" {
		t.Errorf("expected indexName")
	}
	d := ff.Dests()
	if d == nil {
		t.Errorf("expected dests")
	}
	var buf bytes.Buffer
	err = ff.Stats(&buf)
	if err != nil {
		t.Errorf("expected stats to work")
	}
	err = ff.Close()
	if err != nil {
		t.Errorf("expected close to work")
	}
}

func TestNewFilesFeed(t *testing.T) {
	params := ""
	dests := map[string]Dest{}

	l := NewStdLibLog(os.Stderr, "", log.LstdFlags)
	ff, err := NewFilesFeed(nil, "name", "indexName", "sourceName",
		params, dests, false, l)
	if err != nil {
		t.Errorf("expected no err, err: %v", err)
	}
	if ff == nil {
		t.Errorf("expected ff")
	}

	ff, err = NewFilesFeed(nil, "name", "indexName", "sourceName",
		`}bogus{json`, dests, false, l)
	if err == nil || ff != nil {
		t.Errorf("expected err on bogus json")
	}

	ff, err = NewFilesFeed(nil, "name", "indexName", "",
		params, dests, false, l)
	if err == nil || ff != nil {
		t.Errorf("expected err on empty source name")
	}

	ff, err = NewFilesFeed(nil, "name", "indexName", "../../../etc/psswd",
		params, dests, false, l)
	if err == nil || ff != nil {
		t.Errorf("expected err on bad source name")
	}
}

func TestStartFilesFeed(t *testing.T) {
	params := ""
	dests := map[string]Dest{}

	emptyDir, _ := ioutil.TempDir("./tmp", "test")
	defer os.RemoveAll(emptyDir)

	cfg := NewCfgMem()
	meh := &TestMEH{}
	mgr := NewManager(Version, cfg, nil, NewUUID(), nil,
		"", 1, "", ":1000", emptyDir, "some-datasource", meh, nil)
	err := mgr.Start("wanted")
	if err != nil {
		t.Errorf("expected Manager.Start() to work, err: %v", err)
	}

	sourceType := "files"
	sourceDir := mgr.DataDir() +
		string(os.PathSeparator) + "files" +
		string(os.PathSeparator) + "sourceName" +
		string(os.PathSeparator)

	os.MkdirAll(sourceDir, 0700)

	ioutil.WriteFile(sourceDir+"hi.txt", []byte("hello"), 0600)
	ioutil.WriteFile(sourceDir+"bye.txt", []byte("goodbye"), 0600)

	err = StartFilesFeed(mgr, "feedName", "indexName", "indexUUID",
		sourceType, "sourceName", "sourceUUID", params, dests)
	if err != nil {
		t.Errorf("expected no err on StartFilesFeed")
	}

	err = StartFilesFeed(mgr, "feedName", "indexName", "indexUUID",
		sourceType, "sourceName", "sourceUUID", params, dests)
	if err == nil {
		t.Errorf("expected no err StartFilesFeed re-register")
	}

	err = StartFilesFeed(mgr, "feedNameX", "indexName", "indexUUID",
		sourceType, "sourceName", "sourceUUID",
		`{"numPartitions":1}`,
		dests)
	if err != nil {
		t.Errorf("expected no err StartFilesFeed with 100 numPartitions")
	}

	err = StartFilesFeed(mgr, "feedNameY", "indexName", "indexUUID",
		sourceType, "sourceName", "sourceUUID",
		`{"numPartitions":-100}`,
		dests)
	if err != nil {
		t.Errorf("expected no err StartFilesFeed with negative numPartitions")
	}

	err = StartFilesFeed(mgr, "feedName2", "indexName", "indexUUID",
		sourceType, "sourceName/../illegal/../security", "sourceUUID",
		params, dests)
	if err == nil {
		t.Errorf("expected err on StartFilesFeed with bad sourceNamepath")
	}

	// Let the file walkers run a little.
	time.Sleep(100 * time.Millisecond)
}
