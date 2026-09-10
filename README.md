# Daily Work

CLI that turns your GitHub activity into a concise daily Slack update.

Uses a **small local model only** (`llama3.2:3b` via Ollama) — no cloud AI keys.

**Repo:** https://github.com/7AkhilV/daily-work

---

## Install (anyone)

```bash
curl -fsSL https://raw.githubusercontent.com/7AkhilV/daily-work/main/install.sh | bash
```

That installs the latest release binary, sets up Ollama + `llama3.2:3b` (~2GB), and puts `daily-work` on your PATH.

Update later by running the same command again.

### One-time auth

```bash
daily-work auth
```

Classic GitHub PAT scopes: `repo`, `read:user`, `user:email`

### Daily use

```bash
daily-work
daily-work --date 2026-09-09
daily-work activity              # list commits found (no AI)
```

By default only **organization** repos are included (personal repos like this tool’s own repo are skipped).

---

## Dev install (from source)

```bash
git clone git@github.com:7AkhilV/daily-work.git
cd daily-work
./install.sh --from-source
```

On this machine (personal SSH host alias):

```bash
git clone git@github.com-personal:7AkhilV/daily-work.git
```

---

## Requirements

- macOS or Linux
- [Ollama](https://ollama.com) + `llama3.2:3b` (~2GB)
- GitHub PAT (for fetching your activity)

## License

MIT (personal project)
