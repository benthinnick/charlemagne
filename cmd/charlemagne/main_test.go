package main

import "testing"

func TestParseArgs(t *testing.T) {
	got, err := parseArgs([]string{"build", "users", "--config", "x.json"})
	if err != nil {
		t.Fatal(err)
	}
	if got.cmd != "build" || got.name != "users" || got.configPath != "x.json" {
		t.Fatalf("%+v", got)
	}

	got, err = parseArgs([]string{"build", "users", "--stage", "orm"})
	if err != nil {
		t.Fatal(err)
	}
	if got.stage != "orm" || got.from != "" {
		t.Fatalf("%+v", got)
	}

	got, err = parseArgs([]string{"build", "users", "--from=rpc"})
	if err != nil {
		t.Fatal(err)
	}
	if got.from != "rpc" {
		t.Fatalf("%+v", got)
	}

	got, err = parseArgs([]string{"new", "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if got.cmd != "new" || got.name != "demo" || got.configPath != "" {
		t.Fatalf("%+v", got)
	}

	if _, err := parseArgs([]string{"rebuild", "users"}); err == nil {
		t.Fatal("expected usage error for rebuild")
	}
	if _, err := parseArgs([]string{"build", "users", "--stage", "orm", "--from", "rpc"}); err == nil {
		t.Fatal("expected stage/from conflict")
	}
	if _, err := parseArgs([]string{"new", "demo", "--stage", "orm"}); err == nil {
		t.Fatal("expected unknown flag on new")
	}
}
