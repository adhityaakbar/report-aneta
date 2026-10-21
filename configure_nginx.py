#!/usr/bin/env python3
import os
import subprocess

print("=== Configuring Nginx for report.aneta.my.id ===")

nginx_conf = """server {
    listen 80;
    server_name report.aneta.my.id;

    root /var/www/report;
    index index.html;

    location /api/ {
        proxy_pass http://127.0.0.1:8081;
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

# Clean any conflicting configs that might try to listen on 8088 or duplicate report.aneta.my.id
for c in ["/etc/nginx/conf.d/report.aneta.my.id.conf", "/etc/nginx/conf.d/aneta-api-proxy.conf"]:
    if os.path.exists(c):
        os.remove(c)

# Write to sites-available
available_target = "/etc/nginx/sites-available/report.aneta.my.id"
enabled_target = "/etc/nginx/sites-enabled/report.aneta.my.id"

with open(available_target, "w", encoding="utf-8") as f:
    f.write(nginx_conf)

if not os.path.exists(enabled_target):
    os.symlink(available_target, enabled_target)

print(f"[nginx] Configured {available_target} -> {enabled_target}")

# Start or reload Nginx safely
subprocess.run(["nginx", "-t"], check=True)
subprocess.run(["systemctl", "restart", "nginx"], check=True)
print("[nginx] Nginx restarted successfully!")


