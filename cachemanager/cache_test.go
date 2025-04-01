package cachemanager

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

func TestCompression(t *testing.T) {
	data := []byte("QuantumCoinQuantumCoinQuantumCoinQuantumCoinQuantumCoinQuantumCoinQuantumCoinQuantumCoinQuantumCoinQuantumCoinQuantumCoinQuantumCoinQuantumCoinQuantumCoin")
	fmt.Println(data)
	startTime := time.Now()
	compressed, err := compress(data)
	if err != nil {
		fmt.Println(err)
		t.Fatalf("failed compress")
	}
	fmt.Println("compress time", time.Since(startTime))
	startTime = time.Now()
	uncompressed, err := decompress(compressed)
	if err != nil {
		fmt.Println(err)
		t.Fatalf("failed decompress")
	}
	fmt.Println("decompress time", time.Since(startTime))
	if bytes.Equal(data, uncompressed) == false {
		fmt.Println(err)
		t.Fatalf("match failed")
	}
	fmt.Println("uncompressed size", len(data), "compressed size", len(compressed))
}
