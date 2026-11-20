package keypolicy

import (
	"crypto/rand"
	"math/big"
	"testing"
)

func TestCheck(t *testing.T) {
	cases := map[string]Weakness{
		"AAAAAAAAAAAAAAAA": FewDistinct,
		"ABCDABCDABCDABCD": FewDistinct,
		"1212121212121212": FewDistinct,
		"7K3M9QAAAAP2XW4R": RepeatRun,
		"0123456789ABCDEF": SequenceRun,
		"ZYXWV7K3M9Q2P4R8": SequenceRun,
		"K7MQ2X9P4R3WHT6N": "",
		"0NXYVSZQ65TX7SW1": "",
		"AAAB7K3M9QP2XW4R": "",
		"1234K7MQ2X9PWR3T": "",
	}
	for key, want := range cases {
		if got := Check(key); got != want {
			t.Errorf("Check(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestRandomKeysAreRarelyRejected(t *testing.T) {
	rejected := 0
	const trials = 20000
	for i := 0; i < trials; i++ {
		key := make([]byte, 16)
		for j := range key {
			n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(Alphabet))))
			key[j] = Alphabet[n.Int64()]
		}
		if Check(string(key)) != "" {
			rejected++
		}
	}
	if rejected > trials/200 {
		t.Fatalf("%d of %d random keys rejected; rules are too strict", rejected, trials)
	}
}
