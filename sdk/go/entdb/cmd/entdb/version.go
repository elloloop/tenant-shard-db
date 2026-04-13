package main

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Version is the semantic version of the entdb CLI.
const Version = "0.1.0"

// gitSHA returns the short git SHA of the current commit, or "unknown"
// if git is unavailable or not in a repository.
func gitSHA() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// printVersion writes a version banner to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "entdb %s (git %s)\n", Version, gitSHA())
}
