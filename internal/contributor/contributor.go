package contributor

import (
	"github.com/raymonstah/asianamericanswiki/internal/openai"
)

const (
	repo        = "asianamericanswiki"
	repoOwner   = "raymonstah"
	AuthorName  = "asianamericanswiki-bot"
	AuthorEmail = "dne@asianamericans.wiki"
	branchTo    = "main"
)

type Client struct {
	PullRequestService PullRequestService
	OpenAI             *openai.Client
}
