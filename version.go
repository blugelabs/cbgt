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
)

// The cbgt.Version tracks persistence versioning (schema/format of
// persisted data and configuration).  The main.Version from "git
// describe" that's part of an executable command, in contrast, is an
// overall "product" version.  For example, we might introduce new
// UI-only features or fix a UI typo, in which case we'd bump the
// main.Version number; but, if the persisted data/config format was
// unchanged, then the cbgt.Version number should remain unchanged.
//
// NOTE: You *must* update cbgt.Version if you change what's stored in
// the Cfg (such as the JSON/struct definitions or the planning
// algorithms).
const Version = "5.5.0"
const versionKey = "version"

// Returns true if a given version is modern enough to modify the Cfg.
// Older versions (which are running with older JSON/struct definitions
// or planning algorithms) will see false from their checkVersion()'s.
func checkVersion(log Log, cfg Cfg, myVersion string) (bool, error) {
	tries := 0
	for cfg != nil {
		tries += 1
		if tries > 100 {
			return false,
				fmt.Errorf("version: checkVersion too many tries")
		}

		clusterVersion, cas, err := cfg.Get(versionKey, 0)
		if err != nil {
			return false, err
		}

		if clusterVersion == nil {
			// First time initialization, so save myVersion to cfg and
			// retry in case there was a race.
			_, err = cfg.Set(versionKey, []byte(myVersion), cas)
			if err != nil {
				if _, ok := err.(*CfgCASError); ok {
					// Retry if it was a CAS mismatch due to
					// multi-node startup races.
					continue
				}
				return false, fmt.Errorf("version:"+
					" could not save Version to cfg, err: %v", err)
			}
			log.Printf("version: checkVersion, Cfg version updated %s",
				myVersion)
			continue
		}

		// this check is retained to keep the same behaviour of
		// preventing the older versions to override the newer
		// version Cfgs. Now a Cfg version bump happens only when
		// all nodes in cluster are on a given homogeneous version.
		if VersionGTE(myVersion, string(clusterVersion)) == false {
			return false, nil
		}

		if myVersion != string(clusterVersion) {
			bumpVersion, err := VerifyEffectiveClusterVersion(log, cfg, myVersion)
			if err != nil {
				return false, err
			}
			// checkVersion passes even if no bump version is required
			if !bumpVersion {
				log.Printf("version: checkVersion, no bump for current Cfg"+
					" verion: %s", clusterVersion)
				return true, nil
			}

			// Found myVersion is higher than the clusterVersion and
			// all cluster nodes are on the same myVersion, so save
			// myVersion to cfg and retry in case there was a race.
			_, err = cfg.Set(versionKey, []byte(myVersion), cas)
			if err != nil {
				if _, ok := err.(*CfgCASError); ok {
					// Retry if it was a CAS mismatch due to
					// multi-node startup races.
					continue
				}
				return false, fmt.Errorf("version:"+
					" could not update Version in cfg, err: %v", err)
			}
			log.Printf("version: checkVersion, Cfg version updated %s",
				myVersion)
			continue
		}

		return true, nil
	}

	return false, nil
}

// VerifyEffectiveClusterVersion checks the cluster version values, and
// if the cluster contains any node which is lower than the given
// myVersion, then return false
func VerifyEffectiveClusterVersion(log Log, cfg interface{}, myVersion string) (bool, error) {
	// first check with the ns_server for clusterCompatibility value
	// On any errors in retrieving the values there, fallback to
	// nodeDefinitions level version checks
	if rsc, ok := cfg.(VersionReader); ok {
		ccVersion, err := retry(3, rsc.ClusterVersion)
		if err != nil {
			log.Printf("version: RetrieveNsServerCompatibility, err: %v", err)
			goto NODEDEFS_CHECKS
		}

		appVersion, err := CompatibilityVersion(CfgAppVersion)
		if appVersion != ccVersion {
			log.Printf("version: non matching application compatibility "+
				"version: %d and clusterCompatibility version: %d",
				appVersion, ccVersion)
			return false, nil
		}
		if err != nil {
			log.Printf("version: CompatibilityVersion, err: %v", err)
			goto NODEDEFS_CHECKS
		}

		log.Printf("version: clusterCompatibility: %d matches with"+
			" application version: %d", ccVersion, appVersion)
		return true, err
	}

NODEDEFS_CHECKS:
	// fallback in case ns_server checks errors out for unknown reasons
	if cfg, ok := cfg.(Cfg); ok {
		for _, k := range []string{NODE_DEFS_KNOWN, NODE_DEFS_WANTED} {
			key := CfgNodeDefsKey(k)
			v, _, err := cfg.Get(key, 0)
			if err != nil {
				return false, err
			}

			if v == nil {
				// no existing nodes in cluster
				continue
			}

			nodeDefs := &NodeDefs{}
			err = json.Unmarshal(v, nodeDefs)
			if err != nil {
				return false, err
			}

			for _, node := range nodeDefs.NodeDefs {
				if myVersion != node.ImplVersion &&
					VersionGTE(myVersion, node.ImplVersion) {
					log.Printf("version: version: %s lower than myVersion: %s"+
						" found", node.ImplVersion, myVersion)
					return false, nil
				}
			}
		}
	}

	return true, nil
}

func retry(attempts int, f func() (uint64, error)) (val uint64, err error) {
	if val, err = f(); err != nil {
		if attempts > 0 {
			retry(attempts-1, f)
		}
	}
	return val, err
}

var CfgAppVersion = "6.5.0"
