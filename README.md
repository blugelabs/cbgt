cbgt
====

The cbgt project provides a golang library that helps manage
distributed partitions (or data shards) across an elastic cluster of
servers.

#### Documentation

* [![GoDoc](https://godoc.org/github.com/couchbaselabs/cbgt?status.svg)](https://godoc.org/github.com/couchbaselabs/cbgt)
* [REST API Reference](http://labs.couchbase.com/cbft/api-ref/) -
  these REST API Reference docs come from cbft, which uses the cbgt
  library.
* [UI Screenshots](https://github.com/couchbaselabs/cbgt/issues/16) -
  these screenshots come from cbft, which uses the cbgt library.

NOTE: This library initializes math's random seed
(rand.Seed(time.Now().UTC().UnixNano())) for unique id generation.
