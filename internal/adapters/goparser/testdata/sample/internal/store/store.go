package store

type Store struct{ items []int }

func New() *Store { return &Store{} }

func (s *Store) Put(v int) { s.items = append(s.items, v) }

func (s *Store) Len() int { return len(s.items) }

func unrelated() int { return 42 }
