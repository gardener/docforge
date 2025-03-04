package markdown

import (
	"sync"

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/nodeplugins"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/document"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/githubinfo"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/linkvalidator"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/workers/taskqueue"
	"github.com/gardener/docforge/pkg/writers"
)

type Plugin struct {
	docProcessor document.Processor
	ghInfo       githubinfo.GitHubInfo
}

func NewPlugin(workerCount int, failFast bool, wg *sync.WaitGroup, structure []*manifest.Node, rhs registry.Interface, hugo hugo.Hugo, writer writers.Writer, skipLinkValidation bool, validationWorkersCount int, hostsToReport []string, resourceDownloadWorkersCount int, gitInfoWriter writers.Writer) (nodeplugins.Interface, []taskqueue.QueueController, error) {
	var (
		ghInfo      githubinfo.GitHubInfo
		ghInfoTasks taskqueue.QueueController
		err         error
	)
	queues := []taskqueue.QueueController{}
	if gitInfoWriter != nil {
		ghInfo, ghInfoTasks, err = githubinfo.New(resourceDownloadWorkersCount, failFast, wg, rhs, gitInfoWriter)
		if err != nil {
			return nil, nil, err
		}
		queues = append(queues, ghInfoTasks)
	}
	validator, validatorTasks, err := linkvalidator.New(validationWorkersCount, failFast, wg, rhs, hostsToReport)
	if err != nil {
		return nil, nil, err
	}
	docProcessor, docTasks, err := document.New(workerCount, failFast, wg, structure, validator, rhs, hugo, writer, skipLinkValidation)
	return &Plugin{docProcessor, ghInfo}, append(queues, validatorTasks, docTasks), err
}

func (Plugin) Processor() string {
	return "markdown"
}

func (p *Plugin) Process(node *manifest.Node) error {
	p.docProcessor.ProcessNode(node)
	if p.ghInfo != nil {
		p.ghInfo.WriteGitHubInfo(node)
	}
	return nil
}
