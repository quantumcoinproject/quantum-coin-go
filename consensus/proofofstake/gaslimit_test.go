package proofofstake

import (
	"testing"
)

const (
	testMaxGas        = uint64(300000000)
	testFloorGas      = uint64(2100000)
	testBreakglassGas = uint64(30000000)
)

// buildStatus constructs a round-robin status array for the given block number, setting
// the status of the block at each requested distance (1 = most recent, i.e. blockNumber-1).
func buildStatus(blockNumber uint64, byDistance map[uint64]byte) [GAS_LIMIT_WINDOW]byte {
	var arr [GAS_LIMIT_WINDOW]byte
	for d, s := range byDistance {
		arr[(blockNumber-d)%GAS_LIMIT_WINDOW] = s
	}
	return arr
}

// fillRange sets the same status for a contiguous range of distances [from, to].
func fillRange(m map[uint64]byte, from, to uint64, status byte) {
	for d := from; d <= to; d++ {
		m[d] = status
	}
}

func TestComputeBlockGasLimit(t *testing.T) {
	const blockNumber = uint64(1000)

	tests := []struct {
		name      string
		distances map[uint64]byte
		maxGas    uint64
		want      uint64
	}{
		// ----- normal max (300M) -----
		{
			name:      "all ok yields max",
			distances: map[uint64]byte{},
			maxGas:    testMaxGas,
			want:      testMaxGas,
		},
		{
			name:      "round2 band1 (distance 3) hits floor",
			distances: map[uint64]byte{3: GasStatusNilRound2},
			maxGas:    testMaxGas,
			want:      testFloorGas,
		},
		{
			name:      "round2 band1 (distance 6) also hits floor",
			distances: map[uint64]byte{6: GasStatusNilRound2},
			maxGas:    testMaxGas,
			want:      testFloorGas,
		},
		{
			name:      "round2 band2 (distance 9) caps at 2F",
			distances: map[uint64]byte{9: GasStatusNilRound2},
			maxGas:    testMaxGas,
			want:      2 * testFloorGas,
		},
		{
			name:      "round2 band3 (distance 17) caps at 3F",
			distances: map[uint64]byte{17: GasStatusNilRound2},
			maxGas:    testMaxGas,
			want:      3 * testFloorGas,
		},
		{
			name:      "round2 band4 (distance 25) caps at 4F",
			distances: map[uint64]byte{25: GasStatusNilRound2},
			maxGas:    testMaxGas,
			want:      4 * testFloorGas,
		},
		{
			name: "8 round1 nils superlinear ramp",
			distances: func() map[uint64]byte {
				m := map[uint64]byte{}
				fillRange(m, 1, 8, GasStatusNilRound1)
				return m
			}(),
			maxGas: testMaxGas,
			// nilScore 8, drop = 64*1000/256 = 250, gas = 300M - 297.9M*250/1000
			want: 225525000,
		},
		{
			name: "12 round1 nils superlinear ramp",
			distances: func() map[uint64]byte {
				m := map[uint64]byte{}
				fillRange(m, 1, 12, GasStatusNilRound1)
				return m
			}(),
			maxGas: testMaxGas,
			// nilScore 12, drop = 144*1000/256 = 562, gas = 300M - 297.9M*562/1000
			want: 132580200,
		},
		{
			name: "16 round1 nils reach floor",
			distances: func() map[uint64]byte {
				m := map[uint64]byte{}
				fillRange(m, 1, 16, GasStatusNilRound1)
				return m
			}(),
			maxGas: testMaxGas,
			want:   testFloorGas,
		},
		{
			name: "32 round1 nils reach floor",
			distances: func() map[uint64]byte {
				m := map[uint64]byte{}
				fillRange(m, 1, 32, GasStatusNilRound1)
				return m
			}(),
			maxGas: testMaxGas,
			want:   testFloorGas,
		},
		{
			name: "round2 band3 cap but heavy round1 ramp craters to floor",
			distances: func() map[uint64]byte {
				m := map[uint64]byte{}
				fillRange(m, 1, 16, GasStatusNilRound1)
				m[20] = GasStatusNilRound2 // nearest round2 at distance 20 -> band3 cap 3F
				return m
			}(),
			maxGas: testMaxGas,
			// nilScore = 16 + 2 = 18 -> ramp floor; min(floor, 3F) = floor
			want: testFloorGas,
		},
		{
			name: "16 round2 nils outside band1 still floor (ramp dominates cap)",
			distances: func() map[uint64]byte {
				m := map[uint64]byte{}
				fillRange(m, 9, 24, GasStatusNilRound2) // nearest at distance 9 -> band2 cap 2F
				return m
			}(),
			maxGas: testMaxGas,
			want:   testFloorGas,
		},

		// ----- breakglass max (30M): same case set -----
		{
			name:      "breakglass all ok yields breakglass max",
			distances: map[uint64]byte{},
			maxGas:    testBreakglassGas,
			want:      testBreakglassGas,
		},
		{
			name:      "breakglass round2 band1 (distance 3) hits floor",
			distances: map[uint64]byte{3: GasStatusNilRound2},
			maxGas:    testBreakglassGas,
			want:      testFloorGas,
		},
		{
			name:      "breakglass round2 band2 (distance 9) caps at 2F",
			distances: map[uint64]byte{9: GasStatusNilRound2},
			maxGas:    testBreakglassGas,
			want:      2 * testFloorGas,
		},
		{
			name:      "breakglass round2 band3 (distance 17) caps at 3F",
			distances: map[uint64]byte{17: GasStatusNilRound2},
			maxGas:    testBreakglassGas,
			want:      3 * testFloorGas,
		},
		{
			name:      "breakglass round2 band4 (distance 25) caps at 4F",
			distances: map[uint64]byte{25: GasStatusNilRound2},
			maxGas:    testBreakglassGas,
			want:      4 * testFloorGas,
		},
		{
			name: "breakglass 4 round1 nils ramp",
			distances: func() map[uint64]byte {
				m := map[uint64]byte{}
				fillRange(m, 1, 4, GasStatusNilRound1)
				return m
			}(),
			maxGas: testBreakglassGas,
			// drop = 16*1000/256 = 62, gas = 30M - 27.9M*62/1000 = 28,270,200
			want: 28270200,
		},
		{
			name: "breakglass 8 round1 nils ramp",
			distances: func() map[uint64]byte {
				m := map[uint64]byte{}
				fillRange(m, 1, 8, GasStatusNilRound1)
				return m
			}(),
			maxGas: testBreakglassGas,
			// drop = 250, gas = 30M - 27.9M*250/1000 = 23,025,000
			want: 23025000,
		},
		{
			name: "breakglass 12 round1 nils ramp",
			distances: func() map[uint64]byte {
				m := map[uint64]byte{}
				fillRange(m, 1, 12, GasStatusNilRound1)
				return m
			}(),
			maxGas: testBreakglassGas,
			// drop = 562, gas = 30M - 27.9M*562/1000 = 14,320,200
			want: 14320200,
		},
		{
			name: "breakglass 16 round1 nils reach floor",
			distances: func() map[uint64]byte {
				m := map[uint64]byte{}
				fillRange(m, 1, 16, GasStatusNilRound1)
				return m
			}(),
			maxGas: testBreakglassGas,
			want:   testFloorGas,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := buildStatus(blockNumber, tt.distances)
			got := ComputeBlockGasLimit(status, blockNumber, tt.maxGas, testFloorGas)
			if got != tt.want {
				t.Fatalf("ComputeBlockGasLimit = %d, want %d", got, tt.want)
			}
			if got < testFloorGas || got > tt.maxGas {
				t.Fatalf("result %d out of bounds [%d, %d]", got, testFloorGas, tt.maxGas)
			}
		})
	}
}

// TestComputeBlockGasLimitRoundRobin verifies the slot mapping (block % WINDOW) is honored
// for block numbers that wrap the window boundary.
func TestComputeBlockGasLimitRoundRobin(t *testing.T) {
	cases := []uint64{32, 33, 64, 65, 1000, 5319238}
	for _, blockNumber := range cases {
		// A round-2 nil two blocks back (band1) must drive to the floor regardless of wrap.
		floorStatus := buildStatus(blockNumber, map[uint64]byte{2: GasStatusNilRound2})
		if got := ComputeBlockGasLimit(floorStatus, blockNumber, testMaxGas, testFloorGas); got != testFloorGas {
			t.Fatalf("block %d: expected floor for band1 round2, got %d", blockNumber, got)
		}

		// A round-2 nil ten blocks back (band2) must cap at 2F regardless of wrap.
		capStatus := buildStatus(blockNumber, map[uint64]byte{10: GasStatusNilRound2})
		got := ComputeBlockGasLimit(capStatus, blockNumber, testMaxGas, testFloorGas)
		if got != 2*testFloorGas {
			t.Fatalf("block %d: expected 2F cap for band2 round2, got %d", blockNumber, got)
		}
	}
}

func TestComputeBlockGasLimitWarmup(t *testing.T) {
	// Below a full window of history, only existing blocks are considered and the result
	// stays within bounds (no underflow when blockNumber < WINDOW).
	for blockNumber := uint64(0); blockNumber < GAS_LIMIT_WINDOW; blockNumber++ {
		var status [GAS_LIMIT_WINDOW]byte
		got := ComputeBlockGasLimit(status, blockNumber, testMaxGas, testFloorGas)
		if got != testMaxGas {
			t.Fatalf("block %d: warmup all-zero status should yield max %d, got %d", blockNumber, testMaxGas, got)
		}
	}
}

func TestComputeBlockGasLimitMaxEqualsMin(t *testing.T) {
	var status [GAS_LIMIT_WINDOW]byte
	if got := ComputeBlockGasLimit(status, 1000, testFloorGas, testFloorGas); got != testFloorGas {
		t.Fatalf("maxGas == minGas should yield that value, got %d", got)
	}
}

func TestGasNilStatusFromVote(t *testing.T) {
	tests := []struct {
		name     string
		voteType byte
		round    byte
		want     byte
	}{
		{"ok block", byte(VOTE_TYPE_OK), 1, GasStatusOk},
		{"ok block round 0", byte(VOTE_TYPE_OK), 0, GasStatusOk},
		{"nil round 1", byte(VOTE_TYPE_NIL), 1, GasStatusNilRound1},
		{"nil round 0 treated as round1", byte(VOTE_TYPE_NIL), 0, GasStatusNilRound1},
		{"nil round 2", byte(VOTE_TYPE_NIL), 2, GasStatusNilRound2},
		{"nil round 3", byte(VOTE_TYPE_NIL), 3, GasStatusNilRound2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gasNilStatusFromVote(tt.voteType, tt.round); got != tt.want {
				t.Fatalf("gasNilStatusFromVote(%d,%d) = %d, want %d", tt.voteType, tt.round, got, tt.want)
			}
		})
	}
}
