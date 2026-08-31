package main

import (
	"context"
	"os"
	"time"

	"github.com/prunesh/prunesh/internal/updatecheck"
)

func maybeWarnUpdate(args []string) {
	if !updatecheck.ShouldPrompt(version, args, os.Stdin, os.Stderr, os.Getenv) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	maybeOfferUpdate(ctx, defaultUpdateRuntime())
}
