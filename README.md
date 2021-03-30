cbgt
====

NOTE: this is a fork of Couchbase's [cbgt](https://github.com/couchbase/cbgt) library prior to the BSL license change.  It has most of the Couchbase specific references removed, and uses the blugelabs fork of blance.  The unit tests pass, but it is not known to work beyond this.

The cbgt project provides a Go library that helps manage
distributed partitions (or data shards) across an elastic cluster of
servers.

#### Documentation

[![PkgGoDev](https://pkg.go.dev/badge/github.com/blugelabs/cbgt)](https://pkg.go.dev/github.com/blugelabs/cbgt)

NOTE: This library initializes math's random seed
(rand.Seed(time.Now().UTC().UnixNano())) for unique id generation.
