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
	"log"
	"os"
	"testing"
)

func TestCheckVersion(t *testing.T) {
	l := NewStdLibLog(os.Stderr, "", log.LstdFlags)
	ok, err := checkVersion(l, nil, "1.1.0")
	if err != nil || ok {
		t.Errorf("expect nil err and not ok on nil cfg")
	}

	cfg := NewCfgMem()
	ok, err = checkVersion(l, cfg, "1.0.0")
	if err != nil || !ok {
		t.Errorf("expected first version to win in brand new cfg")
	}
	v, _, err := cfg.Get(versionKey, 0)
	if err != nil || string(v) != "1.0.0" {
		t.Errorf("expected first version to persist in brand new cfg")
	}
	ok, err = checkVersion(l, cfg, "1.1.0")
	if err != nil || !ok {
		t.Errorf("expected upgrade version to win")
	}
	v, _, err = cfg.Get(versionKey, 0)
	if err != nil || string(v) != "1.1.0" {
		t.Errorf("expected upgrade version to persist in brand new cfg")
	}
	ok, err = checkVersion(l, cfg, "1.0.0")
	if err != nil || ok {
		t.Errorf("expected lower version to lose")
	}
	v, _, err = cfg.Get(versionKey, 0)
	if err != nil || string(v) != "1.1.0" {
		t.Errorf("expected version to remain stable on lower version check")
	}

	for i := 0; i < 3; i++ {
		cfg = NewCfgMem()
		eac := &ErrorAfterCfg{
			inner:    cfg,
			errAfter: i,
		}
		ok, err = checkVersion(l, eac, "1.0.0")
		if err == nil || ok {
			t.Errorf("expected err when cfg errors on %d'th op", i)
		}
	}

	cfg = NewCfgMem()
	eac := &ErrorAfterCfg{
		inner:    cfg,
		errAfter: 3,
	}
	ok, err = checkVersion(l, eac, "1.0.0")
	if err != nil || !ok {
		t.Errorf("expected ok when cfg doesn't error until 3rd op ")
	}

	cfg = NewCfgMem()
	eac = &ErrorAfterCfg{
		inner:    cfg,
		errAfter: 4,
	}
	ok, err = checkVersion(l, eac, "1.0.0")
	if err != nil || !ok {
		t.Errorf("expected ok on first version init")
	}
	ok, err = checkVersion(l, eac, "1.1.0")
	if err == nil || ok {
		t.Errorf("expected err when forcing cfg Set() error during version upgrade")
	}
}

func TestCheckVersionForUpgrades(t *testing.T) {
	l := NewStdLibLog(os.Stderr, "", log.LstdFlags)
	cfg := NewCfgMem()
	ok, err := checkVersion(l, cfg, "5.0.0")
	if err != nil || !ok {
		t.Errorf("expected first version to win in brand new cfg")
	}

	v, _, err := cfg.Get(versionKey, 0)
	if err != nil || string(v) != "5.0.0" {
		t.Errorf("expected Cfg version 5.0.0")
	}

	// expect to fail as given lower version
	ok, err = checkVersion(l, cfg, "4.5.0")
	if err != nil || ok {
		t.Errorf("expected original version to win against given lower versions, err: %+v", err)
	}

	// case1 - with some old version 5.0.0 nodes
	value := []byte(`{"uuid":"1530042671","nodeDefs":{"710948f76ea4f807dd4e41e44fe74c13":
		{"hostPort":"127.0.0.1:9202","uuid":"710948f76ea4f807dd4e41e44fe74c13",
		"implVersion":"5.0.0","tags":["feed","janitor","pindex","queryer","cbauth_service"],
		"container":"","weight":1,"extras":"{\"features\":\"leanPlan\",\"nsHostPort\":` +
		`\"127.0.0.1:9002\",\"version-cbft.app\":\"v0.5.0\",\"version-cbft.lib\": ` +
		`\"v0.5.0\"}"},"7879038ec4529cc4815f5d927c3df476":{"hostPort":"192.168.1.3:9200",
		"uuid":"7879038ec4529cc4815f5d927c3df476","implVersion":"5.0.0","tags":["feed",
		"janitor","pindex","queryer","cbauth_service"],"container":"","weight":1,
		"extras":"{\"features\":\"leanPlan\",\"nsHostPort\":\"192.168.1.3:9000\", ` +
		`\"version-cbft.app\":\"v0.5.0\",\"version-cbft.lib\":\"v0.5.0\"}"},
		"ecc1c3cad5a58523511e6ff2fd38f6be":{"hostPort":"127.0.0.1:9201",
		"uuid":"ecc1c3cad5a58523511e6ff2fd38f6be","implVersion":"5.0.0",
		"tags":["feed","janitor","pindex","queryer","cbauth_service"],"container":"",
		"weight":1,"extras":"{\"features\":\"leanPlanNext\",\"nsHostPort\":\"127.0.0.1:9001\",` +
		`\"version-cbft.app\":\"v0.5.0\",\"version-cbft.lib\":\"v0.5.0\"}"}},
		"implVersion":"5.0.0"}`)

	for _, k := range []string{NODE_DEFS_KNOWN, NODE_DEFS_WANTED} {
		key := CfgNodeDefsKey(k)
		_, err := cfg.Set(key, value, CFG_CAS_FORCE)
		if err != nil {
			t.Errorf("expected cfg Set to succeed")
		}
	}

	// expected to pass though version bumping not happened with higher version 5.5.0
	ok, err = checkVersion(l, cfg, "5.5.0")
	if err != nil || !ok {
		t.Errorf("expected original version to win until all nodes are on given version, err: %+v", err)
	}

	// original 5.0.0 version retained
	v, _, err = cfg.Get(versionKey, 0)
	if err != nil || string(v) != "5.0.0" {
		t.Errorf("expected Cfg version 5.0.0")
	}

	// case2 - with all cluster nodes on 5.5.0 version
	value = []byte(`{"uuid":"1530042671","nodeDefs":{"710948f76ea4f807dd4e41e44fe74c13":
		{"hostPort":"127.0.0.1:9202","uuid":"710948f76ea4f807dd4e41e44fe74c13",
		"implVersion":"5.5.0","tags":["feed","janitor","pindex","queryer","cbauth_service"],
		"container":"","weight":1,"extras":"{\"features\":\"leanPlan\",\"nsHostPort\":` +
		`\"127.0.0.1:9002\",\"version-cbft.app\":\"v0.5.0\",\"version-cbft.lib\": ` +
		`\"v0.5.0\"}"},"7879038ec4529cc4815f5d927c3df476":{"hostPort":"192.168.1.3:9200",
		"uuid":"7879038ec4529cc4815f5d927c3df476","implVersion":"5.5.0","tags":["feed",
		"janitor","pindex","queryer","cbauth_service"],"container":"","weight":1,
		"extras":"{\"features\":\"leanPlan\",\"nsHostPort\":\"192.168.1.3:9000\", ` +
		`\"version-cbft.app\":\"v0.5.0\",\"version-cbft.lib\":\"v0.5.0\"}"},
		"ecc1c3cad5a58523511e6ff2fd38f6be":{"hostPort":"127.0.0.1:9201",
		"uuid":"ecc1c3cad5a58523511e6ff2fd38f6be","implVersion":"5.5.0",
		"tags":["feed","janitor","pindex","queryer","cbauth_service"],"container":"",
		"weight":1,"extras":"{\"features\":\"leanPlanNext\",\"nsHostPort\":\"127.0.0.1:9001\",` +
		`\"version-cbft.app\":\"v0.5.0\",\"version-cbft.lib\":\"v0.5.0\"}"}},
		"implVersion":"5.5.0"}`)
	for _, k := range []string{NODE_DEFS_KNOWN, NODE_DEFS_WANTED} {
		key := CfgNodeDefsKey(k)
		_, err := cfg.Set(key, value, CFG_CAS_FORCE)
		if err != nil {
			t.Errorf("expected cfg Set to succeed, err: %+v", err)
		}
	}

	// expected to pass as all nodes are on same version 5.5.0
	ok, err = checkVersion(l, cfg, "5.5.0")
	if err != nil || !ok {
		t.Errorf("expected given version to win as all nodes are on given version, err: %+v", err)
	}

	// expect the 5.5.0 version
	v, _, err = cfg.Get(versionKey, 0)
	if err != nil || string(v) != "5.5.0" {
		t.Errorf("expected Cfg version 5.5.0")
	}

	// expected to pass though version bumping not happened with higher 5.5.5 version
	ok, err = checkVersion(l, cfg, "5.5.5")
	if err != nil || !ok {
		t.Errorf("expected original version to win until all nodes are on given version, err: %+v", err)
	}

	// case3 - mixed node cluster with older 5.5.0 version &  5.5.5
	value = []byte(`{"uuid":"1530042671","nodeDefs":{"710948f76ea4f807dd4e41e44fe74c13":
		{"hostPort":"127.0.0.1:9202","uuid":"710948f76ea4f807dd4e41e44fe74c13",
		"implVersion":"5.5.5","tags":["feed","janitor","pindex","queryer","cbauth_service"],
		"container":"","weight":1,"extras":"{\"features\":\"leanPlan\",\"nsHostPort\":` +
		`\"127.0.0.1:9002\",\"version-cbft.app\":\"v0.5.0\",\"version-cbft.lib\": ` +
		`\"v0.5.0\"}"},"7879038ec4529cc4815f5d927c3df476":{"hostPort":"192.168.1.3:9200",
		"uuid":"7879038ec4529cc4815f5d927c3df476","implVersion":"5.5.0","tags":["feed",
		"janitor","pindex","queryer","cbauth_service"],"container":"","weight":1,
		"extras":"{\"features\":\"leanPlan\",\"nsHostPort\":\"192.168.1.3:9000\", ` +
		`\"version-cbft.app\":\"v0.5.0\",\"version-cbft.lib\":\"v0.5.0\"}"},
		"ecc1c3cad5a58523511e6ff2fd38f6be":{"hostPort":"127.0.0.1:9201",
		"uuid":"ecc1c3cad5a58523511e6ff2fd38f6be","implVersion":"5.5.5",
		"tags":["feed","janitor","pindex","queryer","cbauth_service"],"container":"",
		"weight":1,"extras":"{\"features\":\"leanPlanNext\",\"nsHostPort\":\"127.0.0.1:9001\",` +
		`\"version-cbft.app\":\"v0.5.0\",\"version-cbft.lib\":\"v0.5.0\"}"}},
		"implVersion":"5.5.5"}`)

	for _, k := range []string{NODE_DEFS_KNOWN, NODE_DEFS_WANTED} {
		key := CfgNodeDefsKey(k)
		_, err := cfg.Set(key, value, CFG_CAS_FORCE)
		if err != nil {
			t.Errorf("expected cfg Set to succeed, err: %+v", err)
		}
	}

	// expected to pass though version bumping not happened with higher 5.5.5 version
	ok, err = checkVersion(l, cfg, "5.5.5")
	if err != nil || !ok {
		t.Errorf("expected original version to win until all nodes are on given version, err: %+v", err)
	}

	// expected to pass with the same version as that of current
	ok, err = checkVersion(l, cfg, "5.5.0")
	if err != nil || !ok {
		t.Errorf("expected given version to win as few nodes are on given version, err: %+v", err)
	}

	// expect the 5.5.0 version
	v, _, err = cfg.Get(versionKey, 0)
	if err != nil || string(v) != "5.5.0" {
		t.Errorf("expected Cfg version 5.5.0")
	}

	// case3 - all nodes in version 5.5.5
	value = []byte(`{"uuid":"1530042671","nodeDefs":{"710948f76ea4f807dd4e41e44fe74c13":
		{"hostPort":"127.0.0.1:9202","uuid":"710948f76ea4f807dd4e41e44fe74c13",
		"implVersion":"5.5.5","tags":["feed","janitor","pindex","queryer","cbauth_service"],
		"container":"","weight":1,"extras":"{\"features\":\"leanPlan\",\"nsHostPort\":` +
		`\"127.0.0.1:9002\",\"version-cbft.app\":\"v0.5.0\",\"version-cbft.lib\": ` +
		`\"v0.5.0\"}"},"7879038ec4529cc4815f5d927c3df476":{"hostPort":"192.168.1.3:9200",
		"uuid":"7879038ec4529cc4815f5d927c3df476","implVersion":"5.5.5","tags":["feed",
		"janitor","pindex","queryer","cbauth_service"],"container":"","weight":1,
		"extras":"{\"features\":\"leanPlan\",\"nsHostPort\":\"192.168.1.3:9000\", ` +
		`\"version-cbft.app\":\"v0.5.0\",\"version-cbft.lib\":\"v0.5.0\"}"},
		"ecc1c3cad5a58523511e6ff2fd38f6be":{"hostPort":"127.0.0.1:9201",
		"uuid":"ecc1c3cad5a58523511e6ff2fd38f6be","implVersion":"5.5.5",
		"tags":["feed","janitor","pindex","queryer","cbauth_service"],"container":"",
		"weight":1,"extras":"{\"features\":\"leanPlanNext\",\"nsHostPort\":\"127.0.0.1:9001\",` +
		`\"version-cbft.app\":\"v0.5.0\",\"version-cbft.lib\":\"v0.5.0\"}"}},
		"implVersion":"5.5.5"}`)

	for _, k := range []string{NODE_DEFS_KNOWN, NODE_DEFS_WANTED} {
		key := CfgNodeDefsKey(k)
		_, err := cfg.Set(key, value, CFG_CAS_FORCE)
		if err != nil {
			t.Errorf("expected cfg Set to succeed, err: %+v", err)
		}
	}

	// expected to pass as all nodes are on same version 5.5.5
	ok, err = checkVersion(l, cfg, "5.5.5")
	if err != nil || !ok {
		t.Errorf("expected given version to win as all nodes are on given version, err: %+v", err)
	}

	// expected to pass though no version bumping required with higher version
	ok, err = checkVersion(l, cfg, "5.5.9")
	if err != nil || !ok {
		t.Errorf("expected given version to fail until all nodes are on given version, err: %+v", err)
	}

	// expect the 5.5.5 version
	v, _, err = cfg.Get(versionKey, 0)
	if err != nil || string(v) != "5.5.5" {
		t.Errorf("expected Cfg version 5.5.5")
	}

}

func TestVerifyEffectiveClusterVersion(t *testing.T) {
	l := NewStdLibLog(os.Stderr, "", log.LstdFlags)
	cfg := NewCfgMem()
	eac := &ErrorUntilCfg{
		inner:    cfg,
		errUntil: 2,
	}

	rv, err := VerifyEffectiveClusterVersion(l, eac, CfgAppVersion)
	if err != nil {
		t.Errorf("expected no err: %v", err)
	}
	if !rv {
		t.Errorf("expected cluster version to match lean version %s", CfgAppVersion)
	}
}
