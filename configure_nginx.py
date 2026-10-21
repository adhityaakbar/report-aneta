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

# 1. Write to sites-available & enable
available_target = "/etc/nginx/sites-available/report.aneta.my.id"
enabled_target = "/etc/nginx/sites-enabled/report.aneta.my.id"

try:
    with open(available_target, "w", encoding="utf-8") as f:
        f.write(nginx_conf)
    if not os.path.exists(enabled_target):
        os.symlink(available_target, enabled_target)
    print(f"[nginx] Configured {available_target} and symlinked to {enabled_target}")
    subprocess.run(["nginx", "-t"], check=False)
    subprocess.run(["systemctl", "reload", "nginx"], check=False)
except Exception as e:
    print(f"[nginx] Error writing sites-available: {e}")

# 2. Also write to conf.d just in case nginx.conf includes conf.d/*.conf
conf_d_target = "/etc/nginx/conf.d/report.aneta.my.id.conf"
try:
    with open(conf_d_target, "w", encoding="utf-8") as f:
        f.write(nginx_conf)
    print(f"[nginx] Written {conf_d_target}")
except Exception as e:
    print(f"[nginx] Error writing conf.d: {e}")



