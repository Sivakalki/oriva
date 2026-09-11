"""Oriva AI voice service."""

import os
import socket

__version__ = "0.1.0"

# Workaround: this host's IPv6 egress hangs in SYN-SENT against some origins
# (huggingface.co, PyPI, etc.) while IPv4 works fine, which stalls the first
# Whisper/Piper model download for minutes. Force IPv4-only DNS resolution
# process-wide. Set ORIVA_AI_FORCE_IPV4=0 to disable if a deployment target
# doesn't need it.
if os.environ.get("ORIVA_AI_FORCE_IPV4", "1") != "0":
    _orig_getaddrinfo = socket.getaddrinfo

    def _ipv4_only_getaddrinfo(host, port, family=0, type=0, proto=0, flags=0):  # noqa: A002
        return _orig_getaddrinfo(host, port, socket.AF_INET, type, proto, flags)

    socket.getaddrinfo = _ipv4_only_getaddrinfo
