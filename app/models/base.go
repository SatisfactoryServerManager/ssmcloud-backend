package models

type Deletable interface {
	AtomicDelete() error
}
