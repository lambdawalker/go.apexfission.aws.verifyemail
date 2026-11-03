package main

import (
	"fmt"
	infra "github.com/lambdawalker/go.apexfission.aws.verifyemail/infrastructure"
	"github.com/lambdawalker/go.infra.composer/adapters/pulumi/reportpreview"
	"io"
	"os"
)

func run(args []string, stdout io.Writer) error {
	return reportpreview.RunCLI(args, infra.PreviewFeature, "go.apexfission.aws.verifyemail", infra.Preview, stdout)
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
