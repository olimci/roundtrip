// Package sst stores a source syntax tree backed by a token list.
//
// Node pointers and token elements are linked: mutating the token list changes
// the source represented by nodes. Public helpers require non-nil node pointers
// whose Start and End elements belong to the same token list.
package sst
