package domain

import (
	"sync"
	"testing"
)

// The snark picker is shared by every request goroutine; with -race this fails if it
// holds unsynchronised random state.
func TestRandomSnarkPickerIsSafeForConcurrentUse(t *testing.T) {
	pick := RandomSnarkPicker()
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				if pick(RejectionSelfAward, DefaultSnarkLevel) == "" {
					t.Error("empty snark")
				}
				_ = RandomCappedAwardSnark(5, DefaultSnarkLevel)
				_ = defaultWarSnark(vibeFarm, DefaultSnarkLevel)
			}
		}()
	}
	wg.Wait()
}
