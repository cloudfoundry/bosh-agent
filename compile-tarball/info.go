//go:build !linux

package main

func readStemcellSlug() (string, error) { return "apple-gala/1", nil }
