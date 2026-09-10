# Daily Work — Product Requirements Document

**Product Name:** Daily Work  
**Product Type:** Developer CLI Tool  
**Status:** Product Definition / V1 Planning  
**Initial Platform:** macOS, Linux, Windows  
**Primary Language:** Go  
**Primary Data Source:** GitHub  
**AI:** Configurable AI provider  
**Distribution:** Private GitHub repository initially; compiled binaries; optional npm distribution later

---

# 1. Product Overview

Daily Work is a command-line tool that automatically analyzes a developer's GitHub activity for a selected day and converts the technical activity into a concise, human-readable daily work update.

The generated update is designed specifically for posting in a team communication channel such as Slack's `#tech-huddle`.

The tool should not blindly convert commit messages into bullet points. It should analyze commits, pull requests, changed files, and code changes to identify meaningful pieces of work, group related commits, remove technical noise, and generate a summary matching the developer's existing reporting style.

The developer remains in control of the final output.

They can:

- Review the generated summary
- Edit existing items
- Add work that was not represented in GitHub
- Remove incorrect or unnecessary items
- Regenerate the summary
- Copy the final result to the clipboard
- Optionally publish the final result to Slack

**Automatic Slack publishing is explicitly out of scope for the default workflow.**

---

# 2. Problem Statement

Developers often need to post a daily summary of their completed work to a team Slack channel.

The process is currently manual:

1. Remember what was worked on during the day.
2. Go through GitHub commits and pull requests.
3. Determine which commits represent meaningful tasks.
4. Combine multiple commits belonging to the same task.
5. Ignore merge commits, formatting changes, and other noise.
6. Write the summary in a concise format.
7. Remember work that happened outside GitHub.
8. Copy the final summary to Slack.

This is repetitive and can result in:

- Forgotten tasks
- Too many technical commit-level bullets
- Poorly worded updates
- Duplicate tasks
- Time spent reviewing Git history manually

Daily Work automates the difficult parts while keeping the developer responsible for the final report.

---

# 3. Product Goal

The primary goal is:

> Generate a useful daily work summary from GitHub activity in less than a minute, while allowing the developer to quickly correct or supplement the result before posting it.

The ideal workflow is:

```text
daily-work
    ↓
GitHub activity collected
    ↓
Meaningful work identified
    ↓
Related commits grouped
    ↓
AI generates concise summary
    ↓
Developer reviews/edits
    ↓
Copy to clipboard
    ↓
Paste into Slack
```

Optional:

```text
Developer approves
    ↓
Publish to Slack
```

---

# 4. Target Users

## Primary User

Software developers who:

- Use GitHub for their development work
- Regularly report daily work to their team
- Work across multiple repositories
- Make multiple commits for a single task
- Want concise, human-readable updates

## Secondary Users

Other members of the development team who want to use the same CLI.

The tool should therefore support multiple GitHub users without requiring repository-specific configuration.

---

# 5. Core Product Principles

### 5.1 GitHub is the primary source of truth

The tool should obtain work activity from GitHub rather than relying on local Git repositories.

This allows the tool to work across:

- Multiple machines
- Multiple repositories
- Organization-owned repositories
- Private repositories

The repository does not need to belong to the developer personally.

For example:

```text
GitHub Organization
└── Marma-Fintech-Organization
    ├── whistlingcitizen-BE
    ├── QuestAndGames
    └── Suzhi

Developer:
akhil-throughbit
```

The tool identifies commits authored by the authenticated GitHub user regardless of repository ownership.

---

### 5.2 Commit messages are not enough

The tool should use available GitHub context such as:

- Commit message
- Commit author
- Commit timestamp
- Repository
- Changed files
- Diff/patch where available
- Pull request information
- PR title
- PR description
- Related commits

This allows the AI to understand the actual work rather than simply rephrasing commit messages.

---

### 5.3 Group related commits

Multiple commits representing one task should become one work item.

Example:

```text
fix chat persistence
update chat repository
fix chat query
add chat test
```

Should become:

```text
- WC: Fixed chat persistence issue
```

---

### 5.4 Human approval is mandatory before publishing

AI-generated summaries must never automatically appear in the team's Slack channel.

The developer must be able to review and modify the summary.

---

### 5.5 GitHub does not represent all work

The developer may have completed work that does not appear in GitHub.

Therefore, the CLI must allow manually adding tasks.

Example:

```text
- WC: Debugged Redis connection issue
```

can be added even if no corresponding commit exists.

---

# 6. Example Current Reporting Format

The tool should support concise updates similar to:

```text
09-09-2026

- Suzhi: Event gallery API completed for both FE and Admin Panel
- WC: Added title support for scheduled inspect creation
- WC: Fixed chat persistence issue for Chat module
- WC: Fixed Impacts-related bug
- WC: Fixed authentication issue
```

Another example:

```text
08-09-2026

- Suzhi: Events details page public and admin APIs completed
- WC: Marked participants as left when their socket connection is disconnected
- WC: Events highlight APIs completed for public and admin panel
```

The output should prioritize meaningful tasks over implementation-level details.

---

# 7. User Experience

## 7.1 First Run

The user runs:

```bash
daily-work
```

If GitHub is not connected:

```text
GitHub authentication required.

Please authenticate with GitHub.
```

The tool opens or provides the appropriate GitHub authentication flow.

After authentication:

```text
✓ GitHub connected
✓ Account: akhil-throughbit
```

The tool should automatically discover activity from repositories the authenticated user can access.

**No repository selection should be required.**

---

# 8. Daily Workflow

The standard command is:

```bash
daily-work
```

The command assumes the current date.

Example:

```text
$ daily-work

Fetching GitHub activity for 10-09-2026...

✓ Found 8 commits
✓ Found 2 pull requests
✓ Found 3 repositories

Analyzing work...

✓ Summary generated
```

Then the generated summary is displayed.

---

# 9. Interactive Editing

The CLI should provide an interactive interface.

Example:

```text
────────────────────────────────────────
Daily Work — 10-09-2026
────────────────────────────────────────

- Suzhi: Event gallery API completed for both FE and Admin Panel
- WC: Added title support for scheduled inspect creation
- WC: Fixed chat persistence issue for Chat module
- WC: Fixed Impacts-related bug
- WC: Fixed authentication issue

────────────────────────────────────────

What would you like to do?

❯ Edit
  Add task
  Remove task
  Regenerate
  Copy to clipboard
  Publish to Slack
  Exit
```

---

# 10. Edit Functionality

The developer should be able to edit any generated work item.

Example:

```text
Current:

- WC: Fixed chat persistence issue
```

User changes it to:

```text
- WC: Fixed Chat module persistence issue
```

The edited version becomes the final version.

AI should not overwrite manually edited content unless the developer explicitly chooses to regenerate.

---

# 11. Add Task

The user should be able to manually add work.

Example:

```text
Add task:

> WC: Debugged Redis connection issue
```

The item is added to the generated summary.

This is important because not all meaningful work results in a GitHub commit.

---

# 12. Remove Task

The user should be able to remove an incorrectly inferred or irrelevant item.

Example:

```text
Remove:

- WC: Updated dependency versions
```

The item is removed from the final output.

---

# 13. Regenerate

The user can request another AI generation if the initial result is poor.

Example:

```text
Regenerate summary?

This will replace AI-generated items but preserve manually added/edited
items unless the user explicitly chooses otherwise.

[Y/n]
```

The exact behavior should be finalized during implementation.

---

# 14. Clipboard

The final summary should be copied directly to the system clipboard.

Example:

```text
Copy to clipboard

✓ Copied successfully
```

The user can then paste it into Slack.

This is the primary publishing workflow for V1.

---

# 15. Slack Integration

Slack integration is optional.

The developer may configure Slack and use:

```text
Publish to Slack
```

The tool should publish only after explicit user action.

Example:

```text
Publish to #tech-huddle?

[Y/n]
```

After publishing:

```text
✓ Published to #tech-huddle
```

### Important

The tool must NOT:

- Automatically publish at a fixed time
- Automatically publish immediately after generation
- Publish without user confirmation

The user must explicitly choose the Slack publishing action.

---

# 16. GitHub Authentication

The tool needs authenticated GitHub access.

The authentication should identify the current GitHub user.

Example:

```text
Authenticated user:
akhil-throughbit
```

The tool should use that identity when determining authored work.

Repository ownership is irrelevant.

Example:

```text
Repository:
Marma-Fintech-Organization/whistlingcitizen-BE

Repository owner:
Marma-Fintech-Organization

Commit author:
akhil-throughbit
```

The commit should be considered the user's work.

---

# 17. Repository Discovery

Users should NOT manually configure repositories.

The tool should automatically discover relevant activity from repositories accessible to the authenticated GitHub account.

This means a newly joined project can automatically appear in the daily summary without configuration.

Example:

```text
Today:

whistlingcitizen-BE
Suzhi
QuestAndGames
NewProject
```

The user does not need to add `NewProject` manually.

---

# 18. GitHub Activity Filtering

The tool should primarily consider activity authored by the authenticated user.

Potential activity includes:

- Commits
- Pull requests
- PR-related changes
- Meaningful code changes

The system should avoid treating unrelated repository activity as the user's work.

---

# 19. Noise Filtering

The AI should identify and minimize low-value Git activity.

Examples of likely noise:

```text
merge branch
merge main
resolve conflicts
format code
prettier
eslint fixes
dependency lockfile changes
version bump
generated files
minor refactoring
```

These should generally not become separate daily-work items.

However, they may be included when they represent meaningful work.

The AI must use context rather than a hard-coded blacklist alone.

---

# 20. Work Grouping

The system should group related commits.

Example:

```text
Commit 1:
add event gallery API

Commit 2:
add gallery admin endpoint

Commit 3:
add gallery public endpoint

Commit 4:
add gallery DTO

Commit 5:
add gallery tests
```

Output:

```text
- Suzhi: Event gallery API completed for both FE and Admin Panel
```

The system should prefer task-level summaries over commit-level summaries.

---

# 21. Project Identification

The generated work item should identify the project when possible.

Examples:

```text
- Suzhi: ...
- WC: ...
- QuestAndGames: ...
```

The project naming should be configurable if necessary.

For example:

```text
whistlingcitizen-BE → WC
```

This mapping may eventually be stored in configuration.

---

# 22. AI Requirements

The AI should:

- Summarize actual technical work
- Group related changes
- Avoid inventing functionality
- Avoid exaggerating completed work
- Avoid overly technical implementation details
- Use concise language
- Preserve project naming
- Prefer past-tense descriptions
- Produce task-oriented summaries
- Avoid unnecessary explanations

### Example

Input:

```text
fix participant left socket event
update socket disconnect handler
fix participant presence
add participant disconnect test
```

Preferred:

```text
- WC: Fixed participant presence handling when socket connections are disconnected
```

Not:

```text
- Modified socket event listener
- Updated disconnect handler
- Added participant presence logic
- Added tests
```

---

# 23. AI Provider Architecture

The AI layer should be abstracted so the CLI is not tied to one provider.

Possible providers:

- OpenRouter
- Gemini
- OpenAI
- Local Ollama
- Other compatible providers

Example internal interface:

```text
AIProvider
    ├── OpenRouterProvider
    ├── GeminiProvider
    ├── OpenAIProvider
    └── OllamaProvider
```

The initial implementation can support one provider while keeping the interface extensible.

---

# 24. Date Support

Default:

```bash
daily-work
```

means:

```text
today
```

The tool should also support explicit dates.

Example:

```bash
daily-work --date 2026-09-09
```

Potential future support:

```bash
daily-work --from 2026-09-01 --to 2026-09-10
```

---

# 25. CLI Commands

Initial command:

```bash
daily-work
```

Potential command structure:

```bash
daily-work
daily-work auth
daily-work config
daily-work --date 2026-09-09
```

Options may eventually include:

```bash
daily-work --copy
daily-work --publish
daily-work --regenerate
```

However, V1 should prioritize the interactive experience over having many command-line flags.

---

# 26. Configuration

Configuration should be stored locally.

Possible location:

```text
~/.config/daily-work/
```

Configuration may contain:

```text
GitHub account
AI provider
AI model
Slack configuration
Project naming mappings
User preferences
```

Credentials and tokens must not be stored as plain text in ordinary configuration files where avoidable.

The implementation should use an appropriate OS credential/keychain mechanism.

---

# 27. Data Privacy

The tool may process:

- Private repository metadata
- Commit messages
- Source-code diffs
- Pull request information
- Slack credentials

Therefore privacy is a major requirement.

The tool must:

- Request only required GitHub permissions
- Avoid unnecessary repository access
- Never expose GitHub tokens in logs
- Never expose Slack tokens in logs
- Avoid storing source-code diffs unnecessarily
- Clearly identify what data is sent to the selected AI provider
- Support local AI providers where privacy requirements prohibit external AI APIs

---

# 28. Technology Stack

## Core

```text
Go
```

## GitHub

Use a mature GitHub API client for Go.

## CLI

Use a Go CLI framework and terminal UI library where appropriate.

Potential libraries include:

- Cobra
- Bubble Tea
- Lip Gloss

The exact libraries can be selected during implementation.

## Clipboard

Use a cross-platform Go clipboard library.

## Slack

Use Slack's official API from Go or a well-maintained Go SDK.

## AI

Use HTTP/API clients behind an internal provider abstraction.

---

# 29. Distribution

## V1 — Private Distribution

The source code should be maintained in a private GitHub repository.

Example:

```text
Marma-Fintech-Organization/daily-work
```

or another appropriate private repository.

Developers can build or download releases.

---

# 30. Cross-Platform Builds

The project should produce native binaries.

Examples:

```text
daily-work-darwin-arm64
daily-work-darwin-amd64
daily-work-linux-amd64
daily-work-linux-arm64
daily-work-windows-amd64.exe
```

For an Apple Silicon Mac:

```text
daily-work-darwin-arm64
```

is the native executable.

The user does not need Node.js, Python, or Go installed to run the compiled binary.

---

# 31. Installation

Initially, users can download the appropriate binary.

Later, installation can be simplified through:

- Install scripts
- Homebrew
- GitHub Releases
- Package managers

A `.dmg` or `.pkg` installer is optional and not required for a CLI.

---

# 32. Future npm Distribution

The core application does NOT need to be written in Node.js.

If desired, an npm package can later act as a distribution mechanism for the platform-specific Go binary.

Example:

```bash
npm install -g @organization/daily-work
```

The npm package can install the appropriate binary for:

```text
macOS ARM64
macOS x64
Linux x64
Windows x64
```

This is a future distribution option, not a V1 requirement.

---

# 33. Error Handling

The CLI should provide clear errors.

Examples:

```text
GitHub authentication expired.

Run:

daily-work auth
```

or:

```text
Unable to fetch GitHub activity.

Please check your internet connection.
```

or:

```text
No GitHub activity found for 10-09-2026.
```

AI failure should not destroy already collected GitHub information.

---

# 34. Offline / Partial Failure Behavior

If GitHub is unavailable:

```text
✗ Unable to fetch GitHub activity
```

The tool should fail gracefully.

If AI is unavailable after GitHub activity has been collected, the tool may optionally display the raw activity so the user can still manually prepare the update.

The application should avoid losing collected information during transient failures.

---

# 35. Security Requirements

The tool must:

- Never print access tokens
- Never commit credentials to Git
- Never store credentials in `.env` files that are committed
- Use secure local credential storage where possible
- Request minimum required permissions
- Avoid transmitting unnecessary source code to AI providers
- Avoid logging private repository contents

---

# 36. MVP Scope

The Minimum Viable Product should contain:

### Required

- [ ] Go CLI
- [ ] GitHub authentication
- [ ] Identify authenticated GitHub user
- [ ] Fetch user's GitHub activity
- [ ] Automatically discover relevant repositories
- [ ] Fetch commits
- [ ] Fetch useful PR information
- [ ] Analyze changed files/diffs where appropriate
- [ ] AI summarization
- [ ] Group related commits
- [ ] Filter obvious noise
- [ ] Generate project-prefixed bullets
- [ ] Interactive terminal display
- [ ] Edit work items
- [ ] Add manual work items
- [ ] Remove work items
- [ ] Regenerate
- [ ] Copy to clipboard
- [ ] Local configuration
- [ ] Secure credential handling

### Optional MVP

- [ ] Slack authentication
- [ ] Explicit "Publish to Slack" action

---

# 37. Explicitly Out of Scope for V1

The following should NOT be implemented initially:

- Automatic daily Slack posting
- Scheduled jobs
- Web dashboard
- Mobile application
- Team analytics
- Productivity scoring
- Automatic time tracking
- Local Git commit scanning
- Repository selection UI
- Database server
- Cloud-hosted backend
- Multi-tenant SaaS
- Public npm package
- Automatic task assignment
- Automatic Jira/Linear ticket creation

---

# 38. Future Features

Potential future capabilities:

### Team summaries

```text
daily-work team
```

Generate summaries for an entire development team.

### Jira / Linear integration

Match GitHub work to tickets.

### PR-aware reporting

Use PR descriptions and reviews to improve summaries.

### Weekly reports

```bash
daily-work --week
```

Output:

```text
Week of 07-09-2026

WC
- ...
- ...

Suzhi
- ...
```

### AI learning from user edits

If the developer repeatedly changes:

```text
Generated:
Fixed participant socket handling

User:
Fixed participant presence handling when socket connection is disconnected
```

the tool could eventually learn the preferred wording style.

### Local AI

Support models running entirely locally for organizations with strict source-code privacy requirements.

---

# 39. Success Criteria

The MVP is successful if a developer can run:

```bash
daily-work
```

and within approximately one minute:

1. GitHub activity is retrieved.
2. Meaningful work is identified.
3. Related commits are grouped.
4. A concise daily update is generated.
5. The developer can edit it.
6. The developer can add missing work.
7. The final text can be copied to the clipboard.
8. The developer can paste it directly into Slack.

The generated result should require **minimal editing** in normal cases.

---

# 40. Example End-to-End Experience

```text
$ daily-work

Daily Work
────────────────────────────────────────

Date: 10-09-2026
GitHub user: akhil-throughbit

Fetching GitHub activity...

✓ 8 commits
✓ 2 pull requests
✓ 3 repositories

Analyzing...

✓ 5 meaningful work items identified

────────────────────────────────────────

10-09-2026

- Suzhi: Event gallery API completed for both FE and Admin Panel
- WC: Added title support for scheduled inspect creation
- WC: Fixed chat persistence issue for Chat module
- WC: Fixed Impacts-related bug
- WC: Fixed authentication issue

────────────────────────────────────────

❯ Edit
  Add task
  Remove task
  Regenerate
  Copy to clipboard
  Publish to Slack
  Exit
```

Developer selects:

```text
Add task
```

Adds:

```text
- WC: Debugged Redis connection issue
```

Then:

```text
Copy to clipboard
```

Result:

```text
✓ Copied to clipboard
```

The developer pastes it into:

```text
#tech-huddle
```

No automatic publishing occurs.

---

# 41. Product Philosophy

Daily Work should not attempt to replace the developer's judgment.

Its purpose is to remove the tedious parts of daily reporting:

```text
Remembering
     ↓
Searching GitHub
     ↓
Reading commits
     ↓
Understanding diffs
     ↓
Grouping work
     ↓
Writing bullets
```

The tool should automate these steps while leaving:

```text
Review
Edit
Add missing work
Final decision
```

with the developer.

The ideal experience is therefore:

> **GitHub tells the tool what changed. AI helps understand what the work was. The developer decides what actually belongs in the daily update.**

---

# 42. V1 Technical Architecture

```text
                         ┌─────────────────────┐
                         │       GitHub        │
                         │                     │
                         │ Commits             │
                         │ PRs                 │
                         │ Files / Diffs       │
                         └──────────┬──────────┘
                                    │
                                    ▼
                         ┌─────────────────────┐
                         │ GitHub Service      │
                         │                     │
                         │ Authenticate        │
                         │ Fetch activity      │
                         │ Filter user         │
                         └──────────┬──────────┘
                                    │
                                    ▼
                         ┌─────────────────────┐
                         │ Activity Processor  │
                         │                     │
                         │ Deduplicate         │
                         │ Group commits       │
                         │ Remove noise        │
                         └──────────┬──────────┘
                                    │
                                    ▼
                         ┌─────────────────────┐
                         │    AI Service       │
                         │                     │
                         │ Understand changes  │
                         │ Generate work items │
                         └──────────┬──────────┘
                                    │
                                    ▼
                         ┌─────────────────────┐
                         │  Interactive CLI    │
                         │                     │
                         │ Edit                │
                         │ Add                 │
                         │ Remove              │
                         │ Regenerate          │
                         └──────────┬──────────┘
                                    │
                         ┌──────────┴──────────┐
                         ▼                     ▼
                  ┌─────────────┐       ┌─────────────┐
                  │ Clipboard   │       │    Slack    │
                  │             │       │  Optional   │
                  └─────────────┘       └─────────────┘
```

---

# 43. Final V1 Definition

Daily Work is a **private, cross-platform Go CLI** that:

```text
Connects to GitHub
        ↓
Finds the authenticated user's work
        ↓
Automatically discovers relevant repositories
        ↓
Analyzes commits + PR context
        ↓
Groups related changes
        ↓
Filters meaningless Git noise
        ↓
Uses AI to generate concise task summaries
        ↓
Shows the result interactively
        ↓
Allows manual editing/addition/removal
        ↓
Copies the final update to clipboard
        ↓
Optionally publishes to Slack only when explicitly requested
```

The initial distribution mechanism is **private GitHub releases/source code**.

The application should be built as a standalone Go binary, with npm distribution considered later as an additional distribution mechanism rather than as a technical dependency.