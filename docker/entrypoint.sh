#!/bin/sh
# Budgie sandbox entrypoint

export PATH="$HOME/.local/bin:$PATH"
# Copies auth tokens from host while preserving session conversation history

KIRO_DATA_DIR="/root/.local/share/kiro-cli"
AUTH_SOURCE="/auth/data.sqlite3"
TARGET_DB="$KIRO_DATA_DIR/data.sqlite3"

mkdir -p "$KIRO_DATA_DIR"

if [ ! -f "$TARGET_DB" ]; then
    # First run: copy entire database (includes auth + empty conversations)
    cp "$AUTH_SOURCE" "$TARGET_DB" 2>/dev/null
elif [ -f "$AUTH_SOURCE" ]; then
    # Subsequent runs: only update auth tokens, preserve conversations
    sqlite3 "$TARGET_DB" "ATTACH '$AUTH_SOURCE' AS auth_src; \
        DELETE FROM auth_kv; \
        INSERT INTO auth_kv SELECT * FROM auth_src.auth_kv;" 2>/dev/null
fi

exec "$@"
