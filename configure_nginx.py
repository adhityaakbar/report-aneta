#!/usr/bin/env python3
import os
import subprocess

print("=== Investigating Nginx Failure ===")
subprocess.run(["systemctl", "status", "nginx"], check=False)
subprocess.run(["journalctl", "-u", "nginx", "-n", "20", "--no-pager"], check=False)

# Check port 80 usage
subprocess.run(["lsof", "-i:80"], check=False)
subprocess.run(["fuser", "80/tcp"], check=False)

# Test running nginx directly or killing stale nginx master/worker
subprocess.run(["pkill", "-9", "-f", "nginx"], check=False)
subprocess.run(["systemctl", "start", "nginx"], check=False)
subprocess.run(["systemctl", "status", "nginx"], check=False)

