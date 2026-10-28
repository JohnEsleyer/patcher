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
	Line   int
}
