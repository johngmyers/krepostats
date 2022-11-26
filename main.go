package main

import (
	"bytes"
	"flag"
	"os"

	"github.com/johngmyers/krepostats/pkg/krepostats"
	"github.com/sirupsen/logrus"
	"k8s.io/test-infra/prow/github"
)

type options struct {
	TokenPath string
}

func gatherOptions() options {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.StringVar(&o.TokenPath, "github-token-path", "github.token", "Path to the file containing the GitHub OAuth secret.")

	fs.Parse(os.Args[1:])
	return o
}

func main() {
	o := gatherOptions()

	logrus.SetLevel(logrus.InfoLevel)

	token, err := os.ReadFile(o.TokenPath)
	if err != nil {
		logrus.Fatalf("error reading %s: %v", o.TokenPath, err)
	}

	tokenGenerator := func() []byte {
		return token
	}
	censor := func(c []byte) []byte {
		return bytes.ReplaceAll(c, token, []byte("CENSORED"))
	}
	githubClient := github.NewClientWithFields(logrus.Fields{}, tokenGenerator, censor, github.DefaultGraphQLEndpoint, github.DefaultAPIEndpoint)
	githubClient.Throttle(3500, 1000)

	stats := &krepostats.KRepoStats{
		GHC: githubClient,
	}

	stats.Run()

}
