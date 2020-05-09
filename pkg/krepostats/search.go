package krepostats

import "k8s.io/test-infra/prow/github"

type KRepoStats struct {
	TokenGenerator func() []byte
	GHC            github.Client
}

func (k *KRepoStats) Run() {

}
