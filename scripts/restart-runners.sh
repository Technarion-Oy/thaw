#!/bin/bash
# Must be run as root to manage the systemd services

# --- CONFIGURATION ---
RUNNER_USER="github-runner"
# ---------------------

# 1. Enforce root privileges
if [ "$EUID" -ne 0 ]; then
  echo "Error: Please run this script as root (e.g., using sudo)."
  exit 1
fi

RUNNER_BASE_DIR="/home/$RUNNER_USER"
echo "Restarting GitHub Actions runners in $RUNNER_BASE_DIR..."

# 2. Iterate through all runner directories
for RUNNER_DIR in "$RUNNER_BASE_DIR"/runner-*; do
    # Ensure it's a valid directory
    if [ -d "$RUNNER_DIR" ]; then
        echo "----------------------------------------"
        echo "Processing $RUNNER_DIR..."
        cd "$RUNNER_DIR" || continue

        # 3. Use the built-in service wrapper to stop and start
        if [ -x "./svc.sh" ]; then
            echo "Stopping runner..."
            ./svc.sh stop
            
            echo "Starting runner..."
            ./svc.sh start
        else
            echo "Warning: ./svc.sh not found or not executable in $RUNNER_DIR. Skipping."
        fi
    fi
done

echo "----------------------------------------"
echo "All runners have been successfully restarted!"