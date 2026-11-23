package dsa

import "fmt"

type Tuple[K, V any] struct {
	Fst K
	Snd V
}

func NewTuple[K, V any](fst K, snd V) Tuple[K, V] {
	return Tuple[K, V]{Fst: fst, Snd: snd}
}

func (t *Tuple[K, V]) String() string {
	return fmt.Sprintf("(%v, %v)", t.Fst, t.Snd)
}
