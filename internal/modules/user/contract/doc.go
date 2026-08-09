// Package contract defines the public contract interface for the user module.
//
// This is the only package another module may import (rule L1). Everything in
// it is either an interface another module calls, a DTO it renders, or an event
// it reacts to — never an entity, and never anything that would make a caller
// depend on how this module stores its data.
package contract
