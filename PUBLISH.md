# Publish checklist — personal account `7AkhilV`

## SSH on this Mac

Already set up:

- Key: `~/.ssh/id_ed25519_7AkhilV`
- SSH host alias: `github.com-personal` (keeps work key on `github.com` separate)

Remote URL to use:

```text
git@github.com-personal:7AkhilV/daily-work.git
```

## Add the public key to GitHub (required once)

1. Open https://github.com/settings/keys (logged in as **7AkhilV**)
2. **New SSH key** → paste the public key from:

```bash
pbcopy < ~/.ssh/id_ed25519_7AkhilV.pub
```

3. Test:

```bash
ssh -T git@github.com-personal
```

You should see a success message mentioning `7AkhilV`.

## Create + push the public repo

```bash
cd ~/Desktop/daily-work
gh auth login   # choose personal account 7AkhilV
gh repo create 7AkhilV/daily-work --public --source=. --remote=origin --push
```

If `origin` should use the personal SSH host:

```bash
git remote set-url origin git@github.com-personal:7AkhilV/daily-work.git
git push -u origin main
```

## First release (binaries for install.sh)

```bash
git tag v0.1.0
git push origin v0.1.0
```

Then anyone can install with:

```bash
curl -fsSL https://raw.githubusercontent.com/7AkhilV/daily-work/main/install.sh | bash
```
