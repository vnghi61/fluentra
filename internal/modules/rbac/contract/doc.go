// Package contract defines the public contract interface for the rbac module.
//
// This is the only package another module may import (rule L1). The guard,
// the typed permission constants and the role type live here because every
// module calls them; nothing else in this module is importable.
package contract
