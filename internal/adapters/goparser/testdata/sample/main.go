package main

import (
	"fmt"

	"example.com/sample/internal/store"
)

func main() {
	s := store.New()
	if err := run(s); err != nil {
		fmt.Println(err)
	}
}

func run(s *store.Store) error {
	for i := 0; i < 3; i++ {
		if i%2 == 0 && i > 0 || i == 1 {
			s.Put(i)
		}
	}
	switch s.Len() {
	case 0:
		return fmt.Errorf("empty")
	case 1, 2:
	default:
	}
	return nil
}
