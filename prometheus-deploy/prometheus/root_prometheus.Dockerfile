FROM prom/prometheus:master

USER root
COPY ./prometheus/ /etc/prometheus/
ENTRYPOINT ["/bin/sh", "-c"]

COPY run.sh /bin/
RUN chmod +x /bin/run.sh

CMD [ "/bin/run.sh" ]
