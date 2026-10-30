package main

import (
	"strings"
	"testing"
)

func TestGoASTSymbolReplacement(t *testing.T) {
	src := `package main

type Config struct {
	Port int
	Host string
}

func (c *Config) Validate() bool {
	return c.Port > 0
}

func Add(a, b int) int {
	return a + b
}
`

	// 1. Replace struct
	withStruct := `type Config struct {
	Port    int
	Host    string
	Timeout int
}`
	res, err := replaceSymbol(src, "main.go", "Config", withStruct)
	if err != nil {
		t.Fatalf("unexpected error replacing struct: %v", err)
	}
	if !strings.Contains(res, "Timeout int") {
		t.Errorf("struct was not replaced properly:\n%s", res)
	}

	// 2. Replace method with receiver
	withMethod := `func (c *Config) Validate() bool {
	return c.Port > 0 && c.Host != ""
}`
	res, err = replaceSymbol(res, "main.go", "Config.Validate", withMethod)
	if err != nil {
		t.Fatalf("unexpected error replacing method: %v", err)
	}
	if !strings.Contains(res, `c.Host != ""`) {
		t.Errorf("method was not replaced properly:\n%s", res)
	}

	// 3. Replace standalone function
	withFunc := `func Add(a, b int) int {
	return a + b + 1
}`
	res, err = replaceSymbol(res, "main.go", "Add", withFunc)
	if err != nil {
		t.Fatalf("unexpected error replacing function: %v", err)
	}
	if !strings.Contains(res, "return a + b + 1") {
		t.Errorf("func was not replaced properly:\n%s", res)
	}
}

func TestGoASTAmbiguityDetection(t *testing.T) {
	src := `package main

type Server struct{}
func (s *Server) Start() error { return nil }

type Client struct{}
func (c *Client) Start() error { return nil }
`

	// Unqualified "Start" should trigger ambiguity error
	_, err := replaceSymbol(src, "app.go", "Start", "func dummy() {}")
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}
	if !strings.Contains(err.Error(), "multiple declarations") {
		t.Errorf("expected 'multiple declarations' error, got: %v", err)
	}

	// Qualified receiver should cleanly succeed
	res, err := replaceSymbol(src, "app.go", "Server.Start", "func (s *Server) Start() error { return s.init() }")
	if err != nil {
		t.Fatalf("unexpected error with qualified receiver: %v", err)
	}
	if !strings.Contains(res, "return s.init()") {
		t.Errorf("expected Server.Start to be replaced:\n%s", res)
	}
}

func TestTypeScriptASTSymbolReplacement(t *testing.T) {
	src := `import { db } from './db';

export interface User {
	id: string;
	name: string;
}

export class UserService {
	constructor(private client: Client) {}

	async getUser(id: string): Promise<User> {
		return db.find(id);
	}
}

export const calculateTax = (amount: number): number => {
	const dummy = "{ not a brace }";
	return amount * 0.2;
};
`

	// 1. Replace interface
	withIface := `export interface User {
	id: string;
	name: string;
	email: string;
}`
	res, err := replaceSymbol(src, "user.ts", "User", withIface)
	if err != nil {
		t.Fatalf("failed replacing TS interface: %v", err)
	}
	if !strings.Contains(res, "email: string;") {
		t.Errorf("interface replacement failed:\n%s", res)
	}

	// 2. Replace class method with nested braces
	withMethod := `	async getUser(id: string): Promise<User> {
		const cached = cache.get({ key: id });
		return cached || db.find(id);
	}`
	res, err = replaceSymbol(res, "user.ts", "UserService.getUser", withMethod)
	if err != nil {
		t.Fatalf("failed replacing TS method: %v", err)
	}
	if !strings.Contains(res, "const cached = cache.get") {
		t.Errorf("method replacement failed:\n%s", res)
	}

	// 3. Replace arrow function containing fake braces in strings
	withArrow := `export const calculateTax = (amount: number): number => {
	return amount * 0.25;
};`
	res, err = replaceSymbol(res, "user.ts", "calculateTax", withArrow)
	if err != nil {
		t.Fatalf("failed replacing arrow function: %v", err)
	}
	if !strings.Contains(res, "amount * 0.25") {
		t.Errorf("arrow function replacement failed:\n%s", res)
	}
}
