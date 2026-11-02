// Package artifacts lists the isolated verification Lambda commands.
package artifacts

const Module = "github.com/lambdawalker/go.apexfission.aws.verifyemail"

type Lambda struct{ Name, Package string }

func Lambdas() []Lambda {
	return []Lambda{{"signup", Module + "/cmd/signup"}, {"resend", Module + "/cmd/resend"}, {"confirm", Module + "/cmd/confirm"}}
}
