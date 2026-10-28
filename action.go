package main

type ActionType int

const (
	ActionFile ActionType = iota
	ActionReplace
	ActionDelete
)

type Action struct {
	Type   ActionType
	Path   string
	Body   string // For ActionFile
	Search string // For ActionReplace
	With   string // For ActionReplace
	From   string // For targeted range replacement (from pattern)
	To     string // For targeted range replacement (to pattern)
	Symbol string // For targeted symbol replacement
	Lines  string // For explicit line range replacement
	After  string // For anchor insertion after match
	Before string // For anchor insertion before match
	Line   int
}