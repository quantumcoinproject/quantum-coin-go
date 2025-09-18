package rlpx

import (
	"bytes"
	"fmt"
	"github.com/quantumcoinproject/quantum-coin-go/common"
	"testing"
)

var data = common.FromHex("f9020bf90207a00000000000000000000000000000000000000000000000000000000000123123a00000000000000000000000008888f1f195afa192cfee860698584c030f4c9db1a0ef1552a40b7165c3cd773806b9e0c165b75356e0314bf0706f279c729f51e017a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421b90100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000008302000001832fefd8825208845506eb0780a0000000000000000000000000000000000000000000000000000000000000000080a0bd4472abb6659ebe3ee06ee4d7b72a00a9f4d001caca51342001075469aff49888010000000000000080c0")
var compressedData = common.FromHex("1f8b08000000000000fffac9c4fd93897d01033e2064a88ca2a0a3e3e3c7a9eb174e3affae8d6d46840f33bfcfdc8d0bde8b062de12e4c3d7cb6dc826de78383a9db83c31e187a7f28c8579f53343ff081f882b017f2e2d2674297fd6f767d36e9c08fbc688f07d23373d61e604cd2dffa38798b2241053b19f1ba72248066260606c666fdf7379a82385a42d95eb337e08f3906068686057b5d8a566f4b9db7cfee41de93ebdbb518567eb9c078ea54a08902237b48e6fa2f333a60e1da7000100000fffffd860bf40e020000")

func Test_CompressDecompress(t *testing.T) {
	compressed, err := compress(data)
	if err != nil {
		t.Fatalf("failed to compress data: %v", err)
	}
	fmt.Println(common.Bytes2Hex(compressed))

	if bytes.Compare(compressed, compressedData) != 0 {
		t.Fatalf("failed to compress data correctly")
	}

	uncompressed, err := decompress(compressed)
	if err != nil {
		t.Fatalf("failed to decompress data: %v", err)
	}

	if bytes.Compare(uncompressed, data) != 0 {
		t.Fatalf("uncompressed and compressed data don't match")
	}

	fmt.Println("uncompressed:", len(uncompressed), "compressed:", len(compressed), "ratio (higher is better):", float64(len(uncompressed))/float64(len(compressed)))
}

func BenchmarkCompression(b *testing.B) {
	for b.Loop() {
		_, err := compress(data)
		if err != nil {
			b.Fatalf("failed to compress data: %v", err)
		}
	}
}

func BenchmarkDecompression(b *testing.B) {
	for b.Loop() {
		for i := 0; i < b.N; i++ {
			_, err := decompress(compressedData)
			if err != nil {
				b.Fatalf("failed to decompress data: %v", err)
			}
		}
	}
}
