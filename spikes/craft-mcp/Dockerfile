# Spike, throwaway (spikes/craft-mcp/README.md): Aura's box image plus six Craft CLIs on PATH, so
# `aura mcp add <name> --box -- <app>-cli mcp` can mount each one in the identity's box. The
# binaries come from the release tarballs the 2026-10-18 survey downloaded and checked against the
# releases' digests (D:/tmp/storytold-survey.md §4.4); each is checked again here.
#
#   docker build -t aura-sandbox:craft-spike -f spikes/craft-mcp/Dockerfile D:/tmp/storytold-bin/x
FROM aura-sandbox:local
COPY gridcraft-cli pdfcraft-cli wordcraft-cli deckcraft-cli filmcraft-cli photocraft-cli /usr/local/bin/
RUN set -eu; cd /usr/local/bin; \
    printf '%s  %s\n' \
      2bc9bd64c4c19394c270d5f6e223d9c1f630f905e9ad13b7dedb9024ca2a6133 gridcraft-cli \
      073c9c4eb4193cbbf52211d5ad8102969b0731b3294e44c47ab289ac84d2640f pdfcraft-cli \
      c7b20691020a3379b72fcf85c190dbe9599293ace5e8e0a9fbd64af7ff3a53a5 wordcraft-cli \
      0cc2b873ed12b10b4141875bf664585ba5a63db3b24addac37cbd33c716fa2a5 deckcraft-cli \
      9be82529243657526296f320d2400c99b879dd08c02054f681158a37865fd869 filmcraft-cli \
      a6c66d4667d6b8c85bc346b97afa27ec4f3ff568e7b990f66ce6d27d98794dbe photocraft-cli \
    | sha256sum -c -; \
    chmod 0755 gridcraft-cli pdfcraft-cli wordcraft-cli deckcraft-cli filmcraft-cli photocraft-cli
