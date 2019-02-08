package main

import (
	"fmt"
	"net/http"
	"os"
	"storj.io/storj/pkg/cfgstruct"
	"text/tabwriter"

	"github.com/spf13/cobra"

	//"storj.io/storj/pkg/cfgstruct"
)

var (
	prCmd = &cobra.Command{
		Use:   "pr",
		Short: "manage github pull requests",
	}

	listCmd = &cobra.Command{
		Use:     "list",
		Short:   "list github pull requests",
		Aliases: []string{"ls"},
		RunE:    cmdList,
	}

	listCfg GithubCfg
)

func init() {
	rootCmd.AddCommand(prCmd)
	prCmd.AddCommand(listCmd)

	cfgstruct.Bind(listCmd.Flags(), &listCfg, cfgstruct.ConfDir(defaultConfDir))
}

func NewClient(cfg *ClientConfig) *GithubClient {
	httpClient := &http.Client{}

	return &GithubClient{
		config:     cfg,
		httpClient: httpClient,
		baseUrl:    fmt.Sprintf("%s/%s", GithubAPIReposURL, cfg.Repo),
	}
}

func cmdList(cmd *cobra.Command, args []string) error {
	ghClient := NewClient(&listCfg.ClientConfig)

	prs, err := ghClient.ListPRs()
	if err != nil {
		return nil
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	if _, err := fmt.Fprintln(tw, "PR#\tOwner\tTitle\tReviews\tApprovals\tMergable state"); err != nil {
		return err
	}

	for _, pr := range prs {
		reviews, err := pr.Reviews()
		if err != nil {
			return err
		}

		approvals := reviews.Approvals()
		uniqueUserReviews := reviews.UniqueUsers()

		if _, err := fmt.Fprintf(tw, "%d\t%s\t%s\t%d\t%d\t%s",
			pr.Number,
			pr.User.Login,
			pr.Title,
			uniqueUserReviews,
			approvals,
			pr.MergableState,
		); err != nil {
			return err
		}
	}
	return nil
}
