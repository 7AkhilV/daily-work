package cmd

import (
	"context"
	"fmt"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/7AkhilV/daily-work/internal/activity"
	"github.com/7AkhilV/daily-work/internal/ai"
	"github.com/7AkhilV/daily-work/internal/auth"
	"github.com/7AkhilV/daily-work/internal/config"
	ghclient "github.com/7AkhilV/daily-work/internal/github"
	"github.com/7AkhilV/daily-work/internal/summary"
	"github.com/7AkhilV/daily-work/internal/tui"
)

var dateFlag string

func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "daily-work",
		Short: "Generate a daily work summary from GitHub activity",
		Long:  "Daily Work analyzes your GitHub activity and drafts a concise daily update using a local AI model (Ollama).",
		RunE:  runDaily,
	}
	root.PersistentFlags().StringVar(&dateFlag, "date", "", "Date (YYYY-MM-DD). Defaults to today.")
	root.AddCommand(newAuthCmd())
	root.AddCommand(newSetupCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newActivityCmd())
	return root
}

func runDaily(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	day, err := parseDate(dateFlag)
	if err != nil {
		return err
	}

	pat, err := auth.GetGitHubPAT()
	if err != nil {
		fmt.Println("GitHub authentication required.")
		fmt.Println()
		fmt.Println("Run:")
		fmt.Println()
		fmt.Println("  daily-work auth")
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	fmt.Printf("Fetching GitHub activity for %s...\n\n", summary.FormatDate(day))

	client, err := ghclient.NewClient(ctx, pat)
	if err != nil {
		return err
	}
	fmt.Printf("✓ GitHub connected\n")
	fmt.Printf("✓ Account: %s\n\n", client.Login())

	dayAct, err := client.FetchDayActivity(ctx, day, ghclient.FetchOptions{
		IncludePersonalRepos: cfg.IncludePersonalRepos,
	})
	if err != nil {
		return err
	}

	fmt.Printf("✓ Found %d commits\n", len(dayAct.Commits))
	fmt.Printf("✓ Found %d pull requests (used only to discover extra commits)\n", len(dayAct.PRs))
	fmt.Printf("✓ Found %d repositories\n\n", dayAct.RepoCount)

	if len(dayAct.Commits) == 0 && len(dayAct.PRs) == 0 {
		fmt.Printf("No GitHub activity found for %s.\n", summary.FormatDate(day))
		fmt.Println()
		fmt.Println("You can still add tasks manually.")
	}

	proc := activity.Process(dayAct, cfg.ProjectShortName)

	fmt.Printf("Analyzing with local model %s...\n\n", cfg.AI.Model)

	items, usedFallback, err := summarize(ctx, cfg, proc, day)
	if err != nil {
		return err
	}
	if usedFallback {
		fmt.Println("Showing fallback from GitHub activity (you can still edit/add).")
		fmt.Println()
	} else {
		fmt.Println("✓ Summary generated")
		fmt.Println()
	}

	onRegen := func() ([]summary.WorkItem, error) {
		return summarizeOnly(ctx, cfg, proc, day)
	}

	return tui.Run(day, items, onRegen)
}

func summarize(ctx context.Context, cfg config.Config, proc *activity.ProcessedActivity, day time.Time) ([]summary.WorkItem, bool, error) {
	items, err := summarizeOnly(ctx, cfg, proc, day)
	if err != nil {
		fmt.Printf("⚠ Local AI unavailable:\n%v\n\n", err)
		fallback := ai.FallbackFromActivity(proc)
		if len(fallback) == 0 {
			return []summary.WorkItem{}, true, nil
		}
		return fallback, true, nil
	}
	return items, false, nil
}

func summarizeOnly(ctx context.Context, cfg config.Config, proc *activity.ProcessedActivity, day time.Time) ([]summary.WorkItem, error) {
	provider := ai.NewOllama(cfg.AI.Host, cfg.AI.Model)
	items, err := provider.Summarize(ctx, ai.ActivityInput{
		Processed: proc,
		DateLabel: summary.FormatDate(day),
	})
	if err != nil {
		return nil, err
	}
	return ai.PreferRichSummary(items, proc), nil
}

func parseDate(s string) (time.Time, error) {
	now := time.Now()
	if s == "" {
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, now.Location())
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --date %q (want YYYY-MM-DD)", s)
	}
	return t, nil
}

func newAuthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "auth",
		Short: "Save your GitHub Personal Access Token",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Daily Work — GitHub auth")
			fmt.Println(strings.Repeat("─", 40))
			fmt.Println()
			fmt.Println("Create a classic PAT at: https://github.com/settings/tokens")
			fmt.Println("Scopes: repo, read:user, user:email")
			fmt.Println()
			fmt.Print("GitHub PAT (input hidden): ")
			patBytes, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if err != nil {
				return err
			}
			pat := strings.TrimSpace(string(patBytes))
			if pat == "" {
				return fmt.Errorf("PAT is required")
			}

			ctx := context.Background()
			client, err := ghclient.NewClient(ctx, pat)
			if err != nil {
				return fmt.Errorf("could not validate GitHub PAT: %w", err)
			}
			if err := auth.StoreGitHubPAT(pat); err != nil {
				return err
			}
			fmt.Printf("✓ GitHub connected\n")
			fmt.Printf("✓ Account: %s\n\n", client.Login())

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cfg.AI.Provider = "ollama"
			cfg.AI.Model = config.DefaultModel
			if err := config.Save(cfg); err != nil {
				return err
			}

			fmt.Println("Next: set up the small local model (one-time, ~2GB):")
			fmt.Println()
			fmt.Println("  daily-work setup")
			return nil
		},
	}
}

func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Check Ollama and show how to install llama3.2:3b (~2GB)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cfg.AI.Provider = "ollama"
			if cfg.AI.Model == "" || strings.HasPrefix(cfg.AI.Model, "gemini") {
				cfg.AI.Model = config.DefaultModel
			}
			_ = config.Save(cfg)

			fmt.Println("Daily Work — Local AI setup")
			fmt.Println(strings.Repeat("─", 40))
			fmt.Println()
			fmt.Println("Uses only llama3.2:3b (~2GB) via Ollama — no cloud AI, no API keys.")
			fmt.Println()

			provider := ai.NewOllama(cfg.AI.Host, cfg.AI.Model)
			ctx := context.Background()

			if err := provider.Ping(ctx); err != nil {
				fmt.Println("Ollama status: not running")
				fmt.Println()
				fmt.Println("Do this once:")
				fmt.Println()
				fmt.Println("  1) brew install ollama")
				fmt.Println("  2) open -a Ollama   # or: ollama serve")
				fmt.Println("  3) ollama pull llama3.2:3b")
				fmt.Println("  4) daily-work setup   # re-check")
				fmt.Println()
				fmt.Println(err.Error())
				return nil
			}
			fmt.Println("✓ Ollama is running")

			ok, err := provider.HasModel(ctx)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Printf("✗ Model %s not downloaded yet\n\n", cfg.AI.Model)
				fmt.Println("Run once (~2GB, keep only this model to save disk):")
				fmt.Println()
				fmt.Printf("  ollama pull %s\n\n", cfg.AI.Model)
				fmt.Println("Then re-run: daily-work setup")
				return nil
			}

			fmt.Printf("✓ Model ready: %s\n\n", cfg.AI.Model)
			fmt.Println("You're set. Run:")
			fmt.Println()
			fmt.Println("  daily-work")
			return nil
		},
	}
}

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show or edit configuration",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Print current config path and contents (no secrets)",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.Path()
			if err != nil {
				return err
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			fmt.Printf("Path: %s\n\n", path)
			fmt.Printf("AI provider: %s\n", cfg.AI.Provider)
			fmt.Printf("AI model:    %s\n", cfg.AI.Model)
			fmt.Printf("Ollama host: %s\n", cfg.AI.Host)
			fmt.Printf("Personal repos: %v\n", cfg.IncludePersonalRepos)
			fmt.Println("Projects:")
			if len(cfg.Projects) == 0 {
				fmt.Println("  (none — short names are auto-derived)")
			} else {
				for k, v := range cfg.Projects {
					fmt.Printf("  %s → %s\n", k, v)
				}
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "set-project [repo-name] [short-name]",
		Short: "Map a repository name to a short project label",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if cfg.Projects == nil {
				cfg.Projects = map[string]string{}
			}
			cfg.Projects[args[0]] = args[1]
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Printf("✓ %s → %s\n", args[0], args[1])
			return nil
		},
	})
	return cmd
}

func newActivityCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "activity",
		Short: "List commits (and PRs) found for a day — no AI",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			day, err := parseDate(dateFlag)
			if err != nil {
				return err
			}
			pat, err := auth.GetGitHubPAT()
			if err != nil {
				return err
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			client, err := ghclient.NewClient(ctx, pat)
			if err != nil {
				return err
			}
			act, err := client.FetchDayActivity(ctx, day, ghclient.FetchOptions{
				IncludePersonalRepos: cfg.IncludePersonalRepos,
			})
			if err != nil {
				return err
			}

			fmt.Printf("Date: %s\nUser: %s\n", summary.FormatDate(day), act.User)
			if !cfg.IncludePersonalRepos {
				fmt.Println("Filter: organization repos only (personal repos excluded)")
			}
			fmt.Println()
			fmt.Printf("COMMITS (%d) — primary source for the summary\n", len(act.Commits))
			if len(act.Commits) == 0 {
				fmt.Println("(none found)")
			}
			for i, cm := range act.Commits {
				sha := cm.SHA
				if len(sha) > 7 {
					sha = sha[:7]
				}
				flag := ""
				if cm.IsMerge {
					flag = " [merge]"
				}
				fmt.Printf("%d. [%s] %s%s\n   %s  %s\n\n", i+1, cm.RepoFull, cm.Message, flag, sha, cm.Timestamp.Local().Format("15:04"))
			}
			fmt.Printf("PULL REQUESTS (%d) — used only to discover commits on those PRs\n", len(act.PRs))
			for i, pr := range act.PRs {
				fmt.Printf("%d. [%s] #%d %s\n\n", i+1, pr.RepoFull, pr.Number, pr.Title)
			}
			return nil
		},
	}
}

