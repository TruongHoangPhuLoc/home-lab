FROM python:3.9.17-slim-buster

# Run as a non-root user (UID 65532 — the "nonroot" UID used by distroless).
# Required by Trivy/kube-linter and the Kyverno admission policy planned for
# Layer 3 of the security roadmap.
RUN groupadd -g 65532 app && useradd -u 65532 -g 65532 -m app

WORKDIR /app
COPY ./requirements.txt /app
RUN pip3 install --no-cache-dir -r requirements.txt
COPY --chown=app:app . .

USER 65532
EXPOSE 5000
CMD ["flask", "run", "--host", "0.0.0.0"]
