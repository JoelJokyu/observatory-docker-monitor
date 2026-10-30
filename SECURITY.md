# Security and deployment scope

Observatory is intended for trusted local development on one Docker Engine. The supplied release binds the dashboard to 127.0.0.1 and does not publish PostgreSQL. There is no application authentication. Do not expose it directly to your network or the Internet.

The app can read Docker metadata and logs across the mounted Engine. Logs may contain private information. Although Observatory uses read APIs, mounting the socket read-only does not make the Docker API read-only; compromising an app with that socket can affect the Engine/host. The container currently runs as root for standard socket access, with other capabilities dropped, a read-only filesystem and no-new-privileges. These settings do not replace Docker API authorization.

The release is pre-1.0, with no formal security audit or guaranteed support window. Keep Docker and dependencies updated. Retained history/settings and backups should be treated as private data. Do not attach .env files, database dumps or unredacted logs to public issues.

Before the first public release, the maintainer must enable private vulnerability reporting or add a private contact here. Once enabled, use the repository's Security → Report a vulnerability flow for security-sensitive reports. Repository: https://github.com/JoelJokyu/observatory-docker-monitor. Private reporting has not yet been verified as enabled; do not use public issues for sensitive details in the meantime.
