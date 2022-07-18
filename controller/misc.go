package main

import "os"

func envOr(e string, d string) string {
	s := os.Getenv(e)
	if s == "" {
		return d
	}
	return s
}
