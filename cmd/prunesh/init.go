package main

import (
	"fmt"
	"os"

	"github.com/prunesh/prunesh/internal/projectmarker"
)

func runInit() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "prunesh init: %v\n", err)
		os.Exit(1)
	}
	root := projectmarker.ProjectRoot(cwd)

	if err := projectmarker.Create(root); err != nil {
		fmt.Fprintf(os.Stderr, "prunesh init: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("prunesh: marker written to %s/%s\n", root, projectmarker.MarkerName)
}
