package main

import (
	"fmt"
	infra "github.com/lambdawalker/go.apexfission.aws.verifyemail/infrastructure"
	"github.com/lambdawalker/go.infra.composer/adapters/pulumi/reportpreview"
	"os"
)

func main() {
	if err := reportpreview.RunCLI(os.Args[1:], "verifyemail", "go.apexfission.aws.verifyemail", infra.Preview, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
