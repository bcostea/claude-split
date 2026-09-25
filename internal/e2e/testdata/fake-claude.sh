#!/bin/sh
# Fake claude. `auth` subcommands keep a login marker in the config dir; any
# other call records its args and auth-related env for the test to read.
if [ "$1" = "auth" ]; then
  marker="$CLAUDE_CONFIG_DIR/.fake-login"
  case "$2" in
    login)  echo "test@example.com" > "$marker" ;;
    logout) rm -f "$marker" ;;
    status)
      if [ -f "$marker" ]; then
        printf '{"loggedIn":true,"authMethod":"claude.ai","email":"%s"}\n' "$(cat "$marker")"
      else
        printf '{"loggedIn":false,"authMethod":"none"}\n'
        exit 1
      fi ;;
  esac
  exit 0
fi
{
  echo "ARGS:$@"
  echo "CLAUDE_CONFIG_DIR=$CLAUDE_CONFIG_DIR"
  echo "CLAUDE_CODE_OAUTH_TOKEN=$CLAUDE_CODE_OAUTH_TOKEN"
  echo "ANTHROPIC_API_KEY=$ANTHROPIC_API_KEY"
} > "$CLAUDE_TEST_OUT"
