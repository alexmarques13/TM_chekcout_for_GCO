package tm

import (
	"math/rand"
	"time"
)

func HumanDelay(minMs, maxMs int) {
	if maxMs <= minMs {
		time.Sleep(time.Duration(minMs) * time.Millisecond)
		return
	}
	time.Sleep(time.Duration(minMs+rand.Intn(maxMs-minMs+1)) * time.Millisecond)
}

func HumanThink() { HumanDelay(1800, 4200) }
func HumanBrowse() { HumanDelay(3500, 8500) }
func HumanCheckoutStep() { HumanDelay(900, 2200) }
func HumanPaymentStep() { HumanDelay(1200, 2800) }
