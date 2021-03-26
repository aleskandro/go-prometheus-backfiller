#!/bin/sh

/bin/prometheus --config.file=/etc/prometheus/prometheus.yml --storage.tsdb.path=/prometheus \
  --web.console.libraries=/usr/share/prometheus/console_libraries --web.console.templates=/etc/prometheus/consoles \
  --storage.tsdb.allow-overlapping-blocks \
  --storage.tsdb.retention.time=50y $(test -n "$EXTERNAL_URL" && echo "--web.external-url=$EXTERNAL_URL")
