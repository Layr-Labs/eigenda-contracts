package clients

import (
	"context"
	"encoding/hex"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/Layr-Labs/eigenda/core"
	"github.com/Layr-Labs/eigenda/core/meterer"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

const numBins = uint32(3)

func TestNewAccountant(t *testing.T) {
	reservation := &core.ReservedPayment{
		SymbolsPerSecond: 100,
		StartTimestamp:   100,
		EndTimestamp:     200,
		QuorumSplits:     []byte{50, 50},
		QuorumNumbers:    []uint8{0, 1},
	}
	onDemand := &core.OnDemandPayment{
		CumulativePayment: big.NewInt(500),
	}
	reservationWindow := uint32(6)
	pricePerSymbol := uint32(1)
	minNumSymbols := uint32(100)

	privateKey1, err := crypto.GenerateKey()
	assert.NoError(t, err)
	accountId := hex.EncodeToString(privateKey1.D.Bytes())
	accountant := NewAccountant(accountId, reservation, onDemand, reservationWindow, pricePerSymbol, minNumSymbols, numBins)

	assert.NotNil(t, accountant)
	assert.Equal(t, reservation, accountant.reservation)
	assert.Equal(t, onDemand, accountant.onDemand)
	assert.Equal(t, reservationWindow, accountant.reservationWindow)
	assert.Equal(t, pricePerSymbol, accountant.pricePerSymbol)
	assert.Equal(t, minNumSymbols, accountant.minNumSymbols)
	assert.Equal(t, []PeriodRecord{{Index: 0, Usage: 0}, {Index: 1, Usage: 0}, {Index: 2, Usage: 0}}, accountant.periodRecords)
	assert.Equal(t, big.NewInt(0), accountant.cumulativePayment)
}

func TestAccountBlob_Reservation(t *testing.T) {
	reservation := &core.ReservedPayment{
		SymbolsPerSecond: 200,
		StartTimestamp:   100,
		EndTimestamp:     200,
		QuorumSplits:     []byte{50, 50},
		QuorumNumbers:    []uint8{0, 1},
	}
	onDemand := &core.OnDemandPayment{
		CumulativePayment: big.NewInt(500),
	}
	reservationWindow := uint32(5)
	pricePerSymbol := uint32(1)
	minNumSymbols := uint32(100)

	privateKey1, err := crypto.GenerateKey()
	assert.NoError(t, err)
	accountId := hex.EncodeToString(privateKey1.D.Bytes())
	accountant := NewAccountant(accountId, reservation, onDemand, reservationWindow, pricePerSymbol, minNumSymbols, numBins)

	ctx := context.Background()
	symbolLength := uint32(500)
	quorums := []uint8{0, 1}
	now := time.Now().UnixNano()

	header, err := accountant.AccountBlob(ctx, now, symbolLength, quorums)

	assert.NoError(t, err)
	assert.Equal(t, meterer.GetReservationPeriod(time.Now().Unix(), reservationWindow), meterer.GetReservationPeriodByNanosecond(header.Timestamp, reservationWindow))
	assert.Equal(t, big.NewInt(0), header.CumulativePayment)
	assert.Equal(t, isRotation([]uint64{500, 0, 0}, mapRecordUsage(accountant.periodRecords)), true)

	symbolLength = uint32(700)

	now = time.Now().UnixNano()
	header, err = accountant.AccountBlob(ctx, now, symbolLength, quorums)

	assert.NoError(t, err)
	assert.NotEqual(t, uint64(0), header.Timestamp)
	assert.Equal(t, big.NewInt(0), header.CumulativePayment)
	assert.Equal(t, isRotation([]uint64{1200, 0, 200}, mapRecordUsage(accountant.periodRecords)), true)

	// Second call should use on-demand payment
	now = time.Now().UnixNano()
	header, err = accountant.AccountBlob(ctx, now, 300, quorums)

	assert.NoError(t, err)
	assert.NotEqual(t, uint64(0), header.Timestamp)
	assert.Equal(t, big.NewInt(300), header.CumulativePayment)
}

func TestAccountBlob_OnDemand(t *testing.T) {
	reservation := &core.ReservedPayment{
		SymbolsPerSecond: 200,
		StartTimestamp:   100,
		EndTimestamp:     200,
		QuorumSplits:     []byte{50, 50},
		QuorumNumbers:    []uint8{0, 1},
	}
	onDemand := &core.OnDemandPayment{
		CumulativePayment: big.NewInt(1500),
	}
	reservationWindow := uint32(5)
	pricePerSymbol := uint32(1)
	minNumSymbols := uint32(100)

	privateKey1, err := crypto.GenerateKey()
	assert.NoError(t, err)
	accountId := hex.EncodeToString(privateKey1.D.Bytes())
	accountant := NewAccountant(accountId, reservation, onDemand, reservationWindow, pricePerSymbol, minNumSymbols, numBins)

	ctx := context.Background()
	numSymbols := uint32(1500)
	quorums := []uint8{0, 1}
	now := time.Now().UnixNano()
	header, err := accountant.AccountBlob(ctx, now, numSymbols, quorums)
	assert.NoError(t, err)

	expectedPayment := big.NewInt(int64(numSymbols * pricePerSymbol))
	assert.NotEqual(t, uint64(0), header.Timestamp)
	assert.Equal(t, expectedPayment, header.CumulativePayment)
	assert.Equal(t, isRotation([]uint64{0, 0, 0}, mapRecordUsage(accountant.periodRecords)), true)
	assert.Equal(t, expectedPayment, accountant.cumulativePayment)
}

func TestAccountBlob_InsufficientOnDemand(t *testing.T) {
	reservation := &core.ReservedPayment{}
	onDemand := &core.OnDemandPayment{
		CumulativePayment: big.NewInt(500),
	}
	reservationWindow := uint32(60)
	pricePerSymbol := uint32(100)
	minNumSymbols := uint32(100)

	privateKey1, err := crypto.GenerateKey()
	assert.NoError(t, err)
	accountId := hex.EncodeToString(privateKey1.D.Bytes())
	accountant := NewAccountant(accountId, reservation, onDemand, reservationWindow, pricePerSymbol, minNumSymbols, numBins)

	ctx := context.Background()
	numSymbols := uint32(2000)
	quorums := []uint8{0, 1}
	now := time.Now().UnixNano()
	_, err = accountant.AccountBlob(ctx, now, numSymbols, quorums)
	assert.Contains(t, err.Error(), "neither reservation nor on-demand payment is available")
}

func TestAccountBlobCallSeries(t *testing.T) {
	reservation := &core.ReservedPayment{
		SymbolsPerSecond: 200,
		StartTimestamp:   100,
		EndTimestamp:     200,
		QuorumSplits:     []byte{50, 50},
		QuorumNumbers:    []uint8{0, 1},
	}
	onDemand := &core.OnDemandPayment{
		CumulativePayment: big.NewInt(1000),
	}
	reservationWindow := uint32(5)
	pricePerSymbol := uint32(1)
	minNumSymbols := uint32(100)

	privateKey1, err := crypto.GenerateKey()
	assert.NoError(t, err)
	accountId := hex.EncodeToString(privateKey1.D.Bytes())
	accountant := NewAccountant(accountId, reservation, onDemand, reservationWindow, pricePerSymbol, minNumSymbols, numBins)

	ctx := context.Background()
	quorums := []uint8{0, 1}

	now := time.Now().UnixNano()
	// First call: Use reservation
	header, err := accountant.AccountBlob(ctx, now, 800, quorums)
	assert.NoError(t, err)
	timestamp := (time.Duration(header.Timestamp) * time.Nanosecond).Seconds()
	assert.Equal(t, uint64(meterer.GetReservationPeriodByNanosecond(now, reservationWindow)), uint64(timestamp)/uint64(reservationWindow))
	assert.Equal(t, big.NewInt(0), header.CumulativePayment)

	// Second call: Use remaining reservation + overflow
	now = time.Now().UnixNano()
	header, err = accountant.AccountBlob(ctx, now, 300, quorums)
	assert.NoError(t, err)
	timestamp = (time.Duration(header.Timestamp) * time.Nanosecond).Seconds()
	assert.Equal(t, uint64(meterer.GetReservationPeriodByNanosecond(now, reservationWindow)), uint64(timestamp)/uint64(reservationWindow))
	assert.Equal(t, big.NewInt(0), header.CumulativePayment)

	// Third call: Use on-demand
	now = time.Now().UnixNano()
	header, err = accountant.AccountBlob(ctx, now, 500, quorums)
	assert.NoError(t, err)
	assert.NotEqual(t, uint64(0), header.Timestamp)
	assert.Equal(t, big.NewInt(500), header.CumulativePayment)

	// Fourth call: Insufficient on-demand
	now = time.Now().UnixNano()
	_, err = accountant.AccountBlob(ctx, now, 600, quorums)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "neither reservation nor on-demand payment is available")
}

func TestAccountBlob_BinRotation(t *testing.T) {
	reservation := &core.ReservedPayment{
		SymbolsPerSecond: 1000,
		StartTimestamp:   100,
		EndTimestamp:     200,
		QuorumSplits:     []byte{50, 50},
		QuorumNumbers:    []uint8{0, 1},
	}
	onDemand := &core.OnDemandPayment{
		CumulativePayment: big.NewInt(1000),
	}
	reservationWindow := uint32(1) // Set to 1 second for testing
	pricePerSymbol := uint32(1)
	minNumSymbols := uint32(100)

	privateKey1, err := crypto.GenerateKey()
	assert.NoError(t, err)
	accountId := hex.EncodeToString(privateKey1.D.Bytes())
	accountant := NewAccountant(accountId, reservation, onDemand, reservationWindow, pricePerSymbol, minNumSymbols, numBins)

	ctx := context.Background()
	quorums := []uint8{0, 1}

	// First call
	now := time.Now().UnixNano()
	_, err = accountant.AccountBlob(ctx, now, 800, quorums)
	assert.NoError(t, err)
	assert.Equal(t, isRotation([]uint64{800, 0, 0}, mapRecordUsage(accountant.periodRecords)), true)

	// Second call
	now += int64(reservationWindow) * time.Second.Nanoseconds()
	_, err = accountant.AccountBlob(ctx, now, 300, quorums)
	assert.NoError(t, err)
	assert.Equal(t, isRotation([]uint64{800, 300, 0}, mapRecordUsage(accountant.periodRecords)), true)

	// Third call
	_, err = accountant.AccountBlob(ctx, now, 500, quorums)
	assert.NoError(t, err)
	assert.Equal(t, isRotation([]uint64{800, 800, 0}, mapRecordUsage(accountant.periodRecords)), true)
}

func TestConcurrentBinRotationAndAccountBlob(t *testing.T) {
	reservation := &core.ReservedPayment{
		SymbolsPerSecond: 1000,
		StartTimestamp:   100,
		EndTimestamp:     200,
		QuorumSplits:     []byte{50, 50},
		QuorumNumbers:    []uint8{0, 1},
	}
	onDemand := &core.OnDemandPayment{
		CumulativePayment: big.NewInt(1000),
	}
	reservationWindow := uint32(1) // Set to 1 second for testing
	pricePerSymbol := uint32(1)
	minNumSymbols := uint32(100)

	privateKey1, err := crypto.GenerateKey()
	assert.NoError(t, err)
	accountId := hex.EncodeToString(privateKey1.D.Bytes())
	accountant := NewAccountant(accountId, reservation, onDemand, reservationWindow, pricePerSymbol, minNumSymbols, numBins)

	ctx := context.Background()
	quorums := []uint8{0, 1}

	// Start concurrent AccountBlob calls
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			now := time.Now().UnixNano()
			_, err := accountant.AccountBlob(ctx, now, 100, quorums)
			assert.NoError(t, err)
		}()
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// Check final state
	usages := mapRecordUsage(accountant.periodRecords)
	assert.Equal(t, uint64(1000), usages[0]+usages[1]+usages[2])
}

func TestAccountBlob_ReservationWithOneOverflow(t *testing.T) {
	reservation := &core.ReservedPayment{
		SymbolsPerSecond: 200,
		StartTimestamp:   100,
		EndTimestamp:     200,
		QuorumSplits:     []byte{50, 50},
		QuorumNumbers:    []uint8{0, 1},
	}
	onDemand := &core.OnDemandPayment{
		CumulativePayment: big.NewInt(1000),
	}
	reservationWindow := uint32(5)
	pricePerSymbol := uint32(1)
	minNumSymbols := uint32(100)

	privateKey1, err := crypto.GenerateKey()
	assert.NoError(t, err)
	accountId := hex.EncodeToString(privateKey1.D.Bytes())
	accountant := NewAccountant(accountId, reservation, onDemand, reservationWindow, pricePerSymbol, minNumSymbols, numBins)

	ctx := context.Background()
	quorums := []uint8{0, 1}
	now := time.Now().UnixNano()

	// Okay reservation
	header, err := accountant.AccountBlob(ctx, now, 800, quorums)
	assert.NoError(t, err)
	timestamp := (time.Duration(header.Timestamp) * time.Nanosecond).Seconds()
	assert.Equal(t, uint64(meterer.GetReservationPeriodByNanosecond(now, reservationWindow)), uint64(timestamp)/uint64(reservationWindow))
	assert.Equal(t, big.NewInt(0), header.CumulativePayment)
	assert.Equal(t, isRotation([]uint64{800, 0, 0}, mapRecordUsage(accountant.periodRecords)), true)

	// Second call: Allow one overflow
	header, err = accountant.AccountBlob(ctx, now, 500, quorums)
	assert.NoError(t, err)
	assert.Equal(t, big.NewInt(0), header.CumulativePayment)
	assert.Equal(t, isRotation([]uint64{1300, 0, 300}, mapRecordUsage(accountant.periodRecords)), true)

	// Third call: Should use on-demand payment
	now = time.Now().UnixNano()
	header, err = accountant.AccountBlob(ctx, now, 200, quorums)
	assert.NoError(t, err)
	assert.NotEqual(t, uint64(0), header.Timestamp)
	assert.Equal(t, big.NewInt(200), header.CumulativePayment)
	assert.Equal(t, isRotation([]uint64{1300, 0, 300}, mapRecordUsage(accountant.periodRecords)), true)
}

func TestAccountBlob_ReservationOverflowReset(t *testing.T) {
	reservation := &core.ReservedPayment{
		SymbolsPerSecond: 1000,
		StartTimestamp:   100,
		EndTimestamp:     200,
		QuorumSplits:     []byte{50, 50},
		QuorumNumbers:    []uint8{0, 1},
	}
	onDemand := &core.OnDemandPayment{
		CumulativePayment: big.NewInt(1000),
	}
	reservationWindow := uint32(1) // Set to 1 second for testing
	pricePerSymbol := uint32(1)
	minNumSymbols := uint32(100)

	privateKey1, err := crypto.GenerateKey()
	assert.NoError(t, err)
	accountId := hex.EncodeToString(privateKey1.D.Bytes())
	accountant := NewAccountant(accountId, reservation, onDemand, reservationWindow, pricePerSymbol, minNumSymbols, numBins)

	ctx := context.Background()
	quorums := []uint8{0, 1}

	// full reservation
	now := time.Now().UnixNano()
	_, err = accountant.AccountBlob(ctx, now, 1000, quorums)
	assert.NoError(t, err)
	assert.Equal(t, isRotation([]uint64{1000, 0, 0}, mapRecordUsage(accountant.periodRecords)), true)

	// no overflow
	now = time.Now().UnixNano()
	header, err := accountant.AccountBlob(ctx, now, 500, quorums)
	assert.NoError(t, err)
	assert.Equal(t, isRotation([]uint64{1000, 0, 0}, mapRecordUsage(accountant.periodRecords)), true)
	assert.Equal(t, big.NewInt(500), header.CumulativePayment)

	// Wait for next reservation duration
	time.Sleep(time.Duration(reservationWindow) * time.Second)

	// Third call: Should use new bin and allow overflow again
	now = time.Now().UnixNano()
	_, err = accountant.AccountBlob(ctx, now, 500, quorums)
	assert.NoError(t, err)
	assert.Equal(t, isRotation([]uint64{1000, 500, 0}, mapRecordUsage(accountant.periodRecords)), true)
}

func TestQuorumCheck(t *testing.T) {
	tests := []struct {
		name           string
		quorumNumbers  []uint8
		allowedNumbers []uint8
		expectError    bool
		errorMessage   string
	}{
		{
			name:           "valid quorum numbers",
			quorumNumbers:  []uint8{0, 1},
			allowedNumbers: []uint8{0, 1, 2},
			expectError:    false,
		},
		{
			name:           "empty quorum numbers",
			quorumNumbers:  []uint8{},
			allowedNumbers: []uint8{0, 1},
			expectError:    true,
			errorMessage:   "no quorum numbers provided",
		},
		{
			name:           "invalid quorum number",
			quorumNumbers:  []uint8{0, 2},
			allowedNumbers: []uint8{0, 1},
			expectError:    true,
			errorMessage:   "provided quorum number 2 not allowed",
		},
		{
			name:           "empty allowed numbers",
			quorumNumbers:  []uint8{0},
			allowedNumbers: []uint8{},
			expectError:    true,
			errorMessage:   "provided quorum number 0 not allowed",
		},
		{
			name:           "multiple invalid quorums",
			quorumNumbers:  []uint8{2, 3, 4},
			allowedNumbers: []uint8{0, 1},
			expectError:    true,
			errorMessage:   "provided quorum number 2 not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := QuorumCheck(tt.quorumNumbers, tt.allowedNumbers)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMessage)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func mapRecordUsage(records []PeriodRecord) []uint64 {
	return []uint64{records[0].Usage, records[1].Usage, records[2].Usage}
}

func isRotation(arrA, arrB []uint64) bool {
	n := len(arrA)
	if n != len(arrB) {
		return false
	}

	doubleArrA := append(arrA, arrA...)
	// Check if arrB exists in doubleArrA as a subarray
	for i := 0; i < n; i++ {
		match := true
		for j := 0; j < n; j++ {
			if doubleArrA[i+j] != arrB[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
