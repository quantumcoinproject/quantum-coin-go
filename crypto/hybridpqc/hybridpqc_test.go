package hybridpqc

import (
	"bytes"
	"testing"
)

func TestSeedExpansionV1(t *testing.T) {
	var seed1 [BaseSeedSizeV1]byte
	var seed2 [BaseSeedSizeV1]byte
	for i := 0; i < BaseSeedSizeV1; i++ {
		seed1[i] = byte(i)
		seed2[i] = byte(i + 1)
	}
	expandedSeed1, err := ExpandSeedV1(&seed1)
	if err != nil {
		t.Fatalf("failed")
	}

	expandedSeed2, err := ExpandSeedV1(&seed2)
	if err != nil {
		t.Fatalf("failed")
	}

	expandedSeed3, err := ExpandSeedV1(&seed1)
	if err != nil {
		t.Fatalf("failed")
	}

	if bytes.Equal(expandedSeed1[:], expandedSeed3[:]) == false {
		t.Fatalf("expanded seeds do not match")
	}

	if bytes.Equal(expandedSeed1[:], expandedSeed2[:]) == true {
		t.Fatalf("expanded seeds match")
	}
}

func TestSeedExpansionDet(t *testing.T) {
	seedDet := []byte{172, 225, 248, 155, 203, 184, 25, 30, 170, 234, 120, 74, 108, 34, 234, 163, 96, 243, 133, 251, 141, 191, 247, 182, 13, 106, 56, 164, 214, 179, 143, 188, 253, 182, 185, 124, 21, 89, 72, 245, 198, 128, 37, 144, 170, 127, 227, 74, 207, 38, 218, 180, 9, 3, 70, 186, 30, 164, 224, 215, 225, 70, 242, 170, 223, 41, 220, 205, 23, 89, 21, 10, 35, 47, 200, 207, 80, 239, 219, 143, 117, 90, 17, 81, 123, 238, 48, 187, 49, 28, 23, 95, 251, 233, 247, 76}
	expandedSeedDet := []byte{164, 112, 179, 200, 61, 89, 69, 78, 1, 89, 229, 44, 54, 201, 107, 104, 54, 62, 47, 58, 160, 249, 241, 178, 162, 136, 246, 83, 253, 89, 108, 138, 223, 41, 220, 205, 23, 89, 21, 10, 35, 47, 200, 207, 80, 239, 219, 143, 117, 90, 17, 81, 123, 238, 48, 187, 49, 28, 23, 95, 251, 233, 247, 76, 162, 119, 56, 52, 120, 78, 179, 99, 38, 91, 246, 87, 201, 159, 152, 122, 94, 47, 110, 203, 200, 250, 99, 9, 172, 241, 11, 195, 231, 177, 73, 250, 221, 22, 173, 39, 38, 112, 212, 31, 61, 97, 206, 203, 168, 175, 253, 161, 189, 135, 204, 75, 56, 65, 107, 240, 239, 158, 180, 155, 254, 171, 213, 115, 94, 105, 96, 63, 162, 43, 34, 135, 20, 255, 183, 35, 18, 9, 210, 230, 214, 185, 23, 134, 137, 205, 183, 208, 118, 1, 84, 200, 204, 130, 143, 241}
	var seedDet1 [BaseSeedSizeV1]byte
	copy(seedDet1[:], seedDet[:BaseSeedSizeV1])
	expandedResult, err := ExpandSeedV1(&seedDet1)
	if err != nil {
		t.Fatalf("failed")
	}
	if bytes.Compare(expandedSeedDet, expandedResult[:]) != 0 {
		t.Fatalf("expanded seeds do not match")
	}
}

func TestSeedExpansionV2(t *testing.T) {
	var seed1 [BaseSeedSizeV2]byte
	var seed2 [BaseSeedSizeV2]byte
	for i := 0; i < BaseSeedSizeV2; i++ {
		seed1[i] = byte(i)
		seed2[i] = byte(i + 1)
	}
	expandedSeed1, err := ExpandSeedV2(&seed1)
	if err != nil {
		t.Fatalf("failed")
	}

	expandedSeed2, err := ExpandSeedV2(&seed2)
	if err != nil {
		t.Fatalf("failed")
	}

	expandedSeed3, err := ExpandSeedV2(&seed1)
	if err != nil {
		t.Fatalf("failed")
	}

	if bytes.Equal(expandedSeed1[:], expandedSeed3[:]) == false {
		t.Fatalf("expanded seeds do not match")
	}

	if bytes.Equal(expandedSeed1[:], expandedSeed2[:]) == true {
		t.Fatalf("expanded seeds match")
	}
}

func TestSeedExpansionDetV2(t *testing.T) {
	seedDet := []byte{172, 225, 248, 155, 203, 184, 25, 30, 170, 234, 120, 74, 108, 34, 234, 163, 96, 243, 133, 251, 141, 191, 247, 182, 13, 106, 56, 164, 214, 179, 143, 188, 253, 182, 185, 124, 21, 89, 72, 245, 198, 128, 37, 144, 170, 127, 227, 74, 207, 38, 218, 180, 9, 3, 70, 186, 30, 164, 224, 215, 225, 70, 242, 170, 223, 41, 220, 205, 23, 89, 21, 10, 35, 47, 200, 207, 80, 239, 219, 143}
	expandedSeedDet := []byte{54, 219, 157, 38, 119, 159, 79, 114, 81, 252, 31, 241, 142, 98, 132, 125, 162, 109, 205, 91, 13, 193, 206, 236, 159, 102, 148, 52, 59, 183, 199, 115, 10, 153, 148, 193, 82, 33, 97, 46, 43, 56, 79, 121, 193, 34, 121, 168, 92, 192, 61, 112, 120, 206, 158, 150, 40, 232, 244, 53, 36, 161, 94, 105, 161, 221, 67, 204, 255, 194, 27, 194, 81, 241, 152, 201, 30, 160, 132, 213, 186, 197, 6, 208, 126, 146, 221, 185, 75, 175, 79, 235, 14, 190, 165, 179, 203, 45, 27, 94, 251, 201, 228, 76, 67, 222, 135, 13, 64, 221, 205, 188, 100, 103, 206, 9, 27, 123, 142, 245, 1, 162, 254, 132, 254, 78, 33, 29, 187, 209, 203, 205, 199, 226, 41, 188, 1, 106, 18, 227, 18, 126, 192, 180, 231, 72, 211, 19, 125, 55, 90, 226, 88, 153, 94, 109, 96, 80, 129, 71}
	var seedDet1 [BaseSeedSizeV2]byte
	copy(seedDet1[:], seedDet[:BaseSeedSizeV2])
	expandedResult, err := ExpandSeedV2(&seedDet1)
	if err != nil {
		t.Fatalf("failed")
	}
	if bytes.Compare(expandedSeedDet, expandedResult[:]) != 0 {
		t.Fatalf("expanded seeds do not match")
	}
}
