#!/usr/bin/env python3
import glob
import re
import sys

patched = False
for path in glob.glob("/etc/nginx/sites-available/*") + glob.glob("/etc/nginx/conf.d/*"):
    try:
        with open(path, "r", encoding="utf-8") as f:
            content = f.read()
        if "report.aneta.my.id" in content or "server_name" in content:
            if "location /api/" not in content:
                proxy_str = """
    location /api/ {
        proxy_pass http://127.0.0.1:8088;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_read_timeout 300s;
        proxy_connect_timeout 300s;
    }
"""
                new_content = re.sub(r"(server_name[^;]+;)", r"\1" + proxy_str, content, count=1)
                with open(path, "w", encoding="utf-8") as f:
                    f.write(new_content)
                print(f"[nginx] Added /api/ reverse proxy to {path}")
                patched = True
    except Exception as e:
        print(f"[nginx] Error checking {path}: {e}")

if not patched:
    print("[nginx] No matching config needed patch or /api/ already present")
