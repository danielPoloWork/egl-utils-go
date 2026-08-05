# GitHub Repository Setup

The one-time, repo-level configuration that cannot live as a committed file — branch
protection, rulesets, merge strategy, Discussions, Pages, labels, the first milestone. Run
these once, with admin rights, after creating the GitHub repository for `egl-utils-go`.
Everything here reproduces the reference project's GitHub governance; the in-repo automation
(CI, Dependabot, issue forms, CODEOWNERS, release draft) ships as files and needs no setup.

> Prerequisites: the [`gh`](https://cli.github.com/) CLI, authenticated (`gh auth login`),
> and `OWNER=danielPoloWork` / `REPO=egl-utils-go` exported.

```bash
OWNER=danielPoloWork
REPO=egl-utils-go
BRANCH=master
```

## 1. Merge strategy — squash only, PR title/body as the commit

```bash
gh api -X PATCH repos/$OWNER/$REPO \
  -F allow_squash_merge=true -F allow_merge_commit=false -F allow_rebase_merge=false \
  -F delete_branch_on_merge=true \
  -F squash_merge_commit_title=PR_TITLE -F squash_merge_commit_message=PR_BODY
```

This is why the PR title/body is written "as it should read in `git log` forever"
(AGENTS.md §6.4).

## 2. Labels (one type-label per PR)

```bash
# Requires yq. Imports .github/labels.yml idempotently.
yq -o=json '.[]' .github/labels.yml | jq -c . | while read -r l; do
  name=$(jq -r .name <<<"$l"); color=$(jq -r .color <<<"$l"); desc=$(jq -r .description <<<"$l")
  gh label create "$name" --color "$color" --description "$desc" --force
done
```

## 3. Branch protection / ruleset for `master`

Require PRs, a green CI, linear history, and conversation resolution; block direct pushes and
force-pushes. (Agents never push to the default branch — this enforces it server-side.)

```bash
gh api -X PUT repos/$OWNER/$REPO/branches/$BRANCH/protection \
  --input - <<JSON
{
  "required_status_checks": {
    "strict": true,
    "contexts": ["consistency / lint"]
  },
  "enforce_admins": false,
  "required_pull_request_reviews": { "required_approving_review_count": 0 },
  "required_linear_history": true,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "required_conversation_resolution": true,
  "restrictions": null
}
JSON
```

Add the build matrix contexts (e.g. `build / ubuntu-24.04 / …`) to `contexts` once you have
seen their exact names in the first CI run.

> **Renaming a required context is the one edit to avoid.** A required check that no longer
> reports does not fail a pull request — it leaves it waiting forever. Adding a CI *job* also does
> not add its context; that is a separate call to this API (ADR-0054).

### Signatures — what this project requires, and what it does not

Decided in [ADR-0056](../adr/0056-build-time-supply-chain.md) §(e); recorded here because it is a
repository setting, so nothing in the tree can prove it either way.

- **`required_signatures` on `master`: recommended, and free.** The repository is squash-only, and
  GitHub creates and signs the squash commit itself — `68fd847`, `b8c1165` and `51b9310` each report
  `verified=true`, `reason=valid`, committer `GitHub`. So the flag is *already satisfied* by every
  commit on the branch; turning it on closes the paths that remain (an administrator's direct push,
  or a future change of merge strategy) and breaks nothing. An agent's unsigned feature-branch
  commits never land on `master` as themselves.

  ```bash
  gh api -X POST repos/$OWNER/$REPO/branches/$BRANCH/protection/required_signatures
  ```

- **Signed *tags*: deliberately not required.** `release.md` step 8 gives tag creation to the agent
  as carry-through, so requiring signed tags moves tagging to the maintainer and rewrites three
  boundary tables. The benefit is thin because a tag is not the artifact a consumer verifies —
  `sum.golang.org` holds an append-only record of what each published version resolves to. Handing an
  automated process an unattended key would also make the signature attest "a machine with a key did
  this", which is not what a signature claims.

### Actions policy — refuse an unpinned action server-side

`tools/consistency_lint.py`'s `action-pins` check enforces digest pinning inside the repository
(ADR-0056). This is the backstop for anything the lint cannot see, and it is `false` today:

```bash
# Require every `uses:` to name a commit digest. Satisfied by the current tree.
gh api -X PUT repos/$OWNER/$REPO/actions/permissions \
  -F enabled=true -F allowed_actions=all -F sha_pinning_required=true
```

Also worth confirming, because it is a floor a repository administrator can lower without any diff:

```bash
gh api repos/$OWNER/$REPO/actions/permissions/workflow   # expect default_workflow_permissions: read
```

## 4. Discussions, Pages, and the security policy

```bash
# Enable Discussions (questions/ideas; linked from the issue chooser).
gh api -X PATCH repos/$OWNER/$REPO -F has_discussions=true

# GitHub Pages from the docs/ folder on the default branch (optional doc site).
gh api -X POST repos/$OWNER/$REPO/pages \
  -F "source[branch]=$BRANCH" -F "source[path]=/docs" 2>/dev/null \
  || echo "Pages already configured or needs the web UI once."
```

Private vulnerability reporting (the SECURITY.md target) is enabled in the web UI:
**Settings → Code security → Private vulnerability reporting → Enable**.

## 5. Roadmap milestones — seed every `MN — name`

PRs are delivered against the **roadmap milestones** (AGENTS.md §6.4), so seed **all** of them
from [`ROADMAP.md`](../../ROADMAP.md) up front — the board is then complete before milestone-scoped
delivery begins. Each is titled `MN — <name>` (em-dash, matching the `## Milestone N — <name>`
headers) with a professional description from the milestone's Goal.

```bash
# One POST per roadmap milestone — worked example for Milestone 1:
gh api -X POST repos/$OWNER/$REPO/milestones \
  -f title="M1 — Project bootstrap & CI" -f state=open \
  -f description="The thinnest slice that compiles, tests, and ships under the full quality bar."
```

To generate the create-commands for **every** milestone straight from `ROADMAP.md`, the EADOS
factory ships a helper (available in the in-place model): `python
.eados-core/tools/seed_milestones.py ROADMAP.md` prints the exact `gh api` calls — add `--run` to
execute them. Creating a milestone that already exists returns HTTP 422, so the seeder is
safely re-runnable.

## Re-running

Every command here is idempotent or safely re-runnable. Re-run after changing labels, after a
new CI check name should become required, or when onboarding a second collaborator (then bump
`required_approving_review_count` to 1 and add reviewers to CODEOWNERS).
