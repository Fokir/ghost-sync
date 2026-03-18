package hooks

// PostCommitScript returns the shell script content for the post-commit hook.
func PostCommitScript() string {
	return `#!/bin/sh
# ghost-sync post-commit hook (managed by ghost-sync, do not edit)
if ! command -v ghost-sync >/dev/null 2>&1; then
    echo "ghost-sync: binary not found, skipping sync" >&2
    exit 0
fi
ghost-sync push --from-hook 2>/dev/null &
exit 0
`
}

// PostMergeScript returns the shell script content for the post-merge hook.
func PostMergeScript() string {
	return `#!/bin/sh
# ghost-sync post-merge hook (managed by ghost-sync, do not edit)
if ! command -v ghost-sync >/dev/null 2>&1; then
    echo "ghost-sync: binary not found, skipping sync" >&2
    exit 0
fi
ghost-sync pull --from-hook 2>/dev/null
exit 0
`
}
