FROM nginxinc/nginx-unprivileged:1-trixie-otel

COPY web/ /usr/share/nginx/html
COPY web/nginx.conf /etc/nginx/conf.d/default.conf
COPY scripts/generate_config.sh /usr/local/bin/generate_config.sh

EXPOSE 80

CMD ["sh", "/usr/local/bin/generate_config.sh"]
