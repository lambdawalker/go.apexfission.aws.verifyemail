package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/lambdawalker/go.apexfission.aws.verifyemail/api"
	"github.com/lambdawalker/go.apexfission.aws.verifyemail/cmd/internal/bootstrap"
)

func main() {
	handler := api.Handler{Service: bootstrap.Service()}
	lambda.Start(handler.Resend)
}
