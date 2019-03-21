package main

import (
	"github.com/n-inja/tracerr"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(tracerr.Analyzer) }
