package provider

import (
	"context"
	"fmt"
)

type Provider interface {
	Name() string
	Execute(context.Context, Request) (Result, error)
}

type Request struct {
	Input     map[string]any
	Decisions []ChoiceQuestion
}

type ChoiceQuestion struct {
	Name        string
	Instruction string
	Options     map[string]string
}

type Result struct {
	Choices map[string]string
	Scores  map[string]map[string]float64
}

type Set struct {
	providers map[string]Provider
}

func NewSet(providers ...Provider) *Set {
	set := &Set{providers: make(map[string]Provider, len(providers))}
	for _, item := range providers {
		set.providers[item.Name()] = item
	}
	return set
}

func (s *Set) Has(name string) bool {
	_, ok := s.providers[name]
	return ok
}

func (s *Set) Get(name string) (Provider, error) {
	value, ok := s.providers[name]
	if !ok {
		return nil, NewError(name, ErrorUnavailable, 0, fmt.Errorf("provider %q is not configured", name))
	}
	return value, nil
}
