cbgt
====

NOTE: this is a fork of Couchbase's [cbgt](https://github.com/couchbase/cbgt) library prior to the BSL license change.  It has most of the Couchbase specific references removed, and uses the blugelabs fork of blance.  The unit tests pass, but it is not known to work beyond this.

The cbgt project provides a golang library that helps manage
distributed partitions (or data shards) across an elastic cluster of
servers.

#### Documentation

* [![GoDoc](https://godoc.org/github.com/couchbase/cbgt?status.svg)](https://godoc.org/github.com/couchbase/cbgt)
* [REST API Reference](http://labs.couchbase.com/cbft/api-ref/) -
  these REST API Reference docs come from cbft, which uses the cbgt
  library.
* [UI Screenshots](https://github.com/couchbase/cbgt/issues/16) -
  these screenshots come from cbft, which uses the cbgt library.

NOTE: This library initializes math's random seed
(rand.Seed(time.Now().UTC().UnixNano())) for unique id generation.
