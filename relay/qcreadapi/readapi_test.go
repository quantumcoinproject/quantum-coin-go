package qcreadapi

import (
	"fmt"
	"testing"
)

func TestSuite(t *testing.T) {

	// Creating a slice
	myslice := []string{"This", "is", "the", "tutorial",
		"of", "Go", "language"}

	// Iterate slice
	// using range in for loop
	for index, ele := range myslice {
		fmt.Printf("Index = %d and element = %s\n", index, ele)
	}
}
