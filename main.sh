#!/usr/bin/env sh

set -u

if [ $# -lt 1 ]; then
  echo "Usage: $0 <repo>"
  echo
  echo "Arguments:"
  echo "  <repo>          The repository name in the format 'owner/repo'."
  echo "Environment variables:"
  echo "  PR_NUMBER       The pull request number to comment on."
  echo "  USER_LOGIN      The GitHub user login."
  echo "  COMMIT_SHA      The commit SHA."
  echo "  URL             The deployment URL."
  echo "  TITLE           The comment title. Default is '# Preview Deployment'."
  echo "  ASSETS_DIR      Where to look for static assets. Default is '/'."
  echo "  DEBUG           Set to 'true' to enable debug output. Default is 'false'."
  exit 1
fi

if [ -n "${DEBUG:-}" ]; then
  set -x
fi

repo=$1

ASSETS_DIR=${ASSETS_DIR:-}

comment_file=${ASSETS_DIR}/preview-body.json.tpl

title="${TITLE:-# Preview Deployment}"

gh_api() {
  gh api -HX-GitHub-Api-Version:2022-11-28 -Haccept:application/vnd.github.raw+json "$@"
}

process_template() {
    template="$1"
    shift
    result=$(cat "$template")

    # Process each KEY=VALUE pair
    while [ "$#" -gt 0 ]; do
        pair=$1
        key=${pair%%=*}
        value=${pair#*=}
        result=$(echo "$result" | sed "s|{{$key}}|$value|g")
        shift
    done

    echo "$result"
}

comments_url=$(gh_api "/repos/$repo/issues/$PR_NUMBER/comments")

if [ "$comments_url" != '[]' ]; then
  comments_url=$(echo $comments_url | jq -r --arg title "$title" --arg user_login "$USER_LOGIN" '.[] | select((.body | startswith($title)) and (.user.login == $user_login)) | .url' | tr '\n' ' ' | sed 's/ $//')
fi

new_comment_file=$(mktemp)
process_template "$comment_file" TITLE="$title" COMMIT_SHA="$COMMIT_SHA" URL="$URL" > "$new_comment_file"

if [ -n "$comments_url" ]; then
  for comment_url in $comments_url; do
    gh_api -XDELETE "$comment_url"
  done
fi

gh_api -XPOST "/repos/$repo/issues/$PR_NUMBER/comments" --input "$new_comment_file"
