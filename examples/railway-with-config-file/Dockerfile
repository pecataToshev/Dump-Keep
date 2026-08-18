# Layer the skip file on top of the published dump-keep image.
# This lets you version-control your skip list without forking the main repo.
FROM ghcr.io/pecatatoshev/dump-keep:latest

ENV RETENTION="7d,4w,24m"
ENV NOTIFY_TIERS="weekly,monthly"

# Configure version controlled skip list
ENV SKIP_DATABASES_FILE_PATH=/skip.txt
COPY skip.txt /skip.txt
