#!/usr/bin/env python3
import glob
import re
import os
import subprocess

print("=== Checking Nginx Configurations ===")
configs = glob.glob("/etc/nginx/sites-enabled/*") + glob.glob("/etc/nginx/sites-available/*") + glob.glob("/etc/nginx/conf.d/*")
print(f"Discovered configs: {configs}")

for path in set(configs):
    if not os.path.isfile(path):
        continue
    try:
        with open(path, "r", encoding="utf-8") as f:
            content = f.read()
        print(f"Checking {path} (length: {len(content)}):")
        if "location /api/" in content:
            print(f" -> Already has location /api/")
            continue

        if "server {" in content:
            proxy_str = """
    location /api/ {
        proxy_pass http://127.0.0.1:8088;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_read_timeout 300s;
        proxy_connect_timeout 300s;
    }
"""
            new_content = content.replace("server {", "server {" + proxy_str, 1)
            with open(path, "w", encoding="utf-8") as f:
                f.write(new_content)
            print(f" -> Successfully patched {path}!")
    except Exception as e:
        print(f" -> Error reading {path}: {e}")

print("=== Testing Nginx Configuration ===")
subprocess.run(["nginx", "-t"], check=False)

