FROM python:3.11-slim-bookworm

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Clone SIA repository from the mounted workspace or use local copy
ARG SIA_SOURCE_URL=https://github.com/hexo-ai/sia.git
RUN git clone ${SIA_SOURCE_URL} /app/sia 2>/dev/null || true

# Alternative: Copy SIA from local workspace if available
# COPY /home/emage/Code/emage/sia /app/sia

# Install SIA with both backends (claude and openhands)
RUN cd /app/sia && \
    pip install --no-cache-dir --upgrade pip setuptools wheel && \
    pip install --no-cache-dir -e ".[claude,openhands]" || \
    pip install --no-cache-dir -e . && \
    pip install --no-cache-dir python-dotenv

# Copy harness entrypoint script
COPY harness-entrypoint.py /app/harness-entrypoint.py
RUN chmod +x /app/harness-entrypoint.py

# Set environment defaults
ENV PYTHONUNBUFFERED=1
ENV SIA_BACKEND=claude
ENV SIA_MODEL=haiku
ENV SIA_MAX_TURNS=10

# Create /workspace mount point (where CWSO launcher will mount the workspace)
RUN mkdir -p /workspace
WORKDIR /workspace

# Entry point: run the harness adapter
ENTRYPOINT ["/usr/local/bin/python", "/app/harness-entrypoint.py"]
