#!/usr/bin/env python3
import os
import subprocess

nginx_conf = """server {
    listen 80 default_server;
    listen [::]:80 default_server;

    root /var/www/report;
    index index.html;
    server_name _;

    location /api/ {
        proxy_pass http://127.0.0.1:8088;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_read_timeout 300s;
        proxy_connect_timeout 300s;
    }

    location / {
        try_files $uri $uri/ $uri.html /index.html;
    }
}
"""

# Clean any conflicting configs
for c in ["/etc/nginx/conf.d/report.aneta.my.id.conf", "/etc/nginx/conf.d/aneta-api-proxy.conf", "/etc/nginx/sites-available/report.aneta.my.id", "/etc/nginx/sites-enabled/report.aneta.my.id"]:
    try:
        if os.path.islink(c) or os.path.exists(c):
            os.remove(c)
    except Exception:
        pass

# Write clean default
default_target = "/etc/nginx/sites-available/default"
enabled_target = "/etc/nginx/sites-enabled/default"

try:
    with open(default_target, "w", encoding="utf-8") as f:
        f.write(nginx_conf)
    if not os.path.exists(enabled_target):
        os.symlink(default_target, enabled_target)
    print("[nginx] Successfully configured /etc/nginx/sites-available/default")
    subprocess.run(["nginx", "-t"], check=True)
    subprocess.run(["systemctl", "restart", "nginx"], check=False)
    print("[nginx] Nginx restarted successfully!")
except Exception as e:
    print(f"[nginx] Error: {e}")
