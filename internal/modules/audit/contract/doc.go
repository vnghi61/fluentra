// Package contract is the only package of the audit module that another
// module may import (rule L1). It holds the recorder interfaces, the value
// types they take, and nothing else.
//
// Note what is *not* here: an event type. Every arrow in MODULE_INDEX.md §3
// points into `audit`, never out of it, so this module publishes nothing and
// imports no other module's contract. What it consumes it reads as a
// convention over the outbox payload — see service/consumer.go.
package contract
