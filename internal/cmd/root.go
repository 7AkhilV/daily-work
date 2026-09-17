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
		Long:  "Daily Work analyzes your GitHub activity and drafts a concise daily update using Gemini.",
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
	if _, err := auth.GetGeminiKey(); err != nil {
		fmt.Println("Gemini API key required.")
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

	fmt.Printf("Analyzing with Gemini (%s)...\n\n", cfg.AI.Model)

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
		fmt.Printf("⚠ Gemini unavailable:\n%v\n\n", err)
		fallback := ai.FallbackFromActivity(proc)
		if len(fallback) == 0 {
			return []summary.WorkItem{}, true, nil
		}
		return fallback, true, nil
	}
	return items, false, nil
}

func summarizeOnly(ctx context.Context, cfg config.Config, proc *activity.ProcessedActivity, day time.Time) ([]summary.WorkItem, error) {
	key, err := auth.GetGeminiKey()
	if err != nil {
		return nil, err
	}
	provider := ai.NewGemini(key, cfg.AI.Model)
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

func readHidden(prompt string) (string, error) {
	fmt.Print(prompt)
	b, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func newAuthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "auth",
		Short: "Save GitHub PAT and Gemini API key",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Daily Work — auth")
			fmt.Println(strings.Repeat("─", 40))
			fmt.Println()
			fmt.Println("1) GitHub classic PAT: https://github.com/settings/tokens")
			fmt.Println("   Scopes: repo, read:user, user:email")
			fmt.Println()
			pat, err := readHidden("GitHub PAT (input hidden): ")
			if err != nil {
				return err
			}
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
			fmt.Printf("✓ GitHub connected (%s)\n\n", client.Login())

			fmt.Println("2) Gemini API key: https://aistudio.google.com/apikey")
			fmt.Println()
			key, err := readHidden("Gemini API key (input hidden): ")
			if err != nil {
				return err
			}
			if key == "" {
				return fmt.Errorf("Gemini API key is required")
			}
			if err := auth.StoreGeminiKey(key); err != nil {
				return err
			}
			fmt.Println("✓ Gemini key saved")
			fmt.Println()

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cfg.AI.Provider = "gemini"
			cfg.AI.Model = config.DefaultModel
			if err := config.Save(cfg); err != nil {
				return err
			}

			fmt.Println("You're set. Run:")
			fmt.Println()
			fmt.Println("  daily-work")
			return nil
		},
	}
}

func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Check GitHub + Gemini credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cfg.AI.Provider = "gemini"
			if cfg.AI.Model == "" || strings.HasPrefix(cfg.AI.Model, "llama") {
				cfg.AI.Model = config.DefaultModel
			}
			_ = config.Save(cfg)

			fmt.Println("Daily Work — setup")
			fmt.Println(strings.Repeat("─", 40))
			fmt.Println()
			fmt.Printf("AI: Gemini (%s)\n\n", cfg.AI.Model)

			if _, err := auth.GetGitHubPAT(); err != nil {
				fmt.Println("✗ GitHub PAT missing")
			} else {
				fmt.Println("✓ GitHub PAT found")
			}
			if _, err := auth.GetGeminiKey(); err != nil {
				fmt.Println("✗ Gemini API key missing")
				fmt.Println()
				fmt.Println("Get a free key: https://aistudio.google.com/apikey")
				fmt.Println()
				fmt.Println("Then run: daily-work auth")
				return nil
			}
			fmt.Println("✓ Gemini API key found")
			fmt.Println()
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
	cmd.AddCommand(&cobra.Command{
		Use:   "set-model [model]",
		Short: "Set Gemini model (default gemini-3.5-flash)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cfg.AI.Provider = "gemini"
			cfg.AI.Model = args[0]
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Printf("✓ model → %s\n", args[0])
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
