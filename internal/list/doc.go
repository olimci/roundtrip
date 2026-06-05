// Package list provides a small intrusive doubly linked list.
//
// List methods require non-nil *List receivers. Element parameters must be
// non-nil elements currently owned by the receiver unless the method documents
// another contract. Splicing a source list transfers its elements and empties
// the source list.
package list
