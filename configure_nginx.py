#!/usr/bin/env python3
import os
import subprocess

nginx_conf = """# Managed by Aneta Reports Deploy Workflow
server {
    listen 80;
    server_name report.aneta.my.id;

    root /var/www/report;
    index index.html;

    # Go API Backend Reverse Proxy
    location /api/ {
        proxy_pass http://127.0.0.1:8088;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 300s;
        proxy_connect_timeout 300s;
    }

    # Static Routes
    location / {
        try_files $uri $uri/ $uri.html /index.html;
    }
}
"""

target = "/etc/nginx/conf.d/report.aneta.my.id.conf"
try:
    with open(target, "w", encoding="utf-8") as f:
        f.write(nginx_conf)
    print(f"[nginx] Written clean vhost config to {target}")
    subprocess.run(["nginx", "-t"], check=False)
    subprocess.run(["systemctl", "reload", "nginx"], check=False)
    print("[nginx] Reloaded nginx successfully")
except Exception as e:
    print(f"[nginx] Failed to configure: {e}")


