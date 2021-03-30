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

package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/blugelabs/cbgt"
)

var ErrorBindHttp = errors.New("main_cfg:" +
	" couchbase cfg needs network/IP address for bindHttp,\n" +
	"  Please specify a network/IP address for the -bindHttp parameter\n" +
	"  (non-loopback, non-127.0.0.1/localhost, non-0.0.0.0)\n" +
	"  so that this node can be clustered with other nodes.")

// MainCfg connects to a Cfg provider as a server peer (e.g., as a
// cbgt.Manager).
func MainCfg(baseName, connect, bindHttp,
	register, dataDir string) (cbgt.Cfg, error) {
	return MainCfgEx(baseName, connect, bindHttp, register, dataDir, "", nil)
}

// MainCfgEx connects to a Cfg provider as a server peer (e.g., as a
// cbgt.Manager), with more options.
func MainCfgEx(baseName, connect, bindHttp,
	register, dataDir, uuid string, options map[string]string) (cbgt.Cfg, error) {
	// TODO: One day, the default cfg provider should not be simple.
	// TODO: One day, Cfg provider lookup should be table driven.
	var cfg cbgt.Cfg
	var err error
	switch {
	case connect == "", connect == "simple":
		cfg, err = MainCfgSimple(baseName, connect, bindHttp, register, dataDir)
	default:
		err = fmt.Errorf("main_cfg1: unsupported cfg connect: %s", connect)
	}
	return cfg, err
}

// ------------------------------------------------

func MainCfgSimple(baseName, connect, bindHttp, register, dataDir string) (
	cbgt.Cfg, error) {
	cfgPath := dataDir + string(os.PathSeparator) + baseName + ".cfg"
	cfgPathExists := false
	if _, err := os.Stat(cfgPath); err == nil {
		cfgPathExists = true
	}

	cfg := cbgt.NewCfgSimple(cfgPath)
	if cfgPathExists {
		err := cfg.Load()
		if err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

// ------------------------------------------------

// MainCfgClient helper function connects to a Cfg provider as a
// client (not as a known cbgt.Manager server or peer).  This is
// useful, for example, for tool developers as opposed to server
// developers.
func MainCfgClient(baseName, cfgConnect string) (cbgt.Cfg, error) {
	bindHttp := "<NO-BIND-HTTP>"
	register := "unchanged"
	dataDir := "<NO-DATA-DIR>"

	cfg, err := MainCfg(baseName, cfgConnect,
		bindHttp, register, dataDir)
	if err == ErrorBindHttp {
		return nil, err
	}

	if err != nil {
		return nil, fmt.Errorf("main: could not start cfg,"+
			" cfgConnect: %s, err: %v\n"+
			"  Please check that your -cfg/-cfgConnect parameter (%q)\n"+
			"  is correct and/or that your configuration provider\n"+
			"  is available.",
			cfgConnect, err, cfgConnect)
	}

	return cfg, err
}
